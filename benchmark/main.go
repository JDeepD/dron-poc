package main

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/jdeepd/dron-poc/common"
	pb "github.com/jdeepd/dron-poc/dron_poc"
)

type BenchmarkResult struct {
	TotalTasks       int
	Concurrency      int
	SuccessfulCreate int64
	FailedCreate     int64
	TotalDuration    time.Duration
	CreateThroughput float64
	AvgCreateLatency float64
}

type E2EResult struct {
	TotalTasks       int
	TasksCreated     int
	TasksCompleted   int
	TasksFailed      int
	CreateDuration   time.Duration
	CompleteDuration time.Duration
	TotalE2EDuration time.Duration
	CreateThroughput float64
	E2EThroughput    float64
	AvgE2ELatency    time.Duration
}

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	coordAddr := flag.String("coordinator", common.DefaultCoordinatorAddress, "Coordinator address")
	numTasks := flag.Int("tasks", 100, "Number of tasks to create")
	concurrency := flag.Int("concurrency", 10, "Number of concurrent clients")
	waitForCompletion := flag.Bool("wait", false, "Wait and poll for task completion (requires workers)")
	timeout := flag.Duration("timeout", 60*time.Second, "Timeout for waiting for task completion")
	flag.Parse()

	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              DRON Performance Benchmark                    ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Printf("\nCoordinator:  %s\n", *coordAddr)
	fmt.Printf("Tasks:        %d\n", *numTasks)
	fmt.Printf("Concurrency:  %d\n", *concurrency)
	fmt.Printf("E2E mode:     %v\n", *waitForCompletion)
	if *waitForCompletion {
		fmt.Printf("Timeout:      %v\n", *timeout)
	}
	fmt.Println("\n─────────────────────────────────────────────────────────────")

	conn, err := grpc.NewClient(*coordAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to coordinator")
	}
	defer conn.Close()

	client := pb.NewCoordinatorServiceClient(conn)

	if *waitForCompletion {
		fmt.Println("\n[E2E Benchmark] Full Task Lifecycle")
		fmt.Println("────────────────────────────────────")
		fmt.Println("⚠️  Make sure workers are running!")
		fmt.Println()
		result := benchmarkEndToEnd(client, *numTasks, *concurrency, *timeout)
		printE2EResults(result)
	} else {
		fmt.Println("\n[Test] CreateTask RPC Throughput")
		fmt.Println("─────────────────────────────────")
		result := benchmarkCreateTask(client, *numTasks, *concurrency)
		printResults(result)
	}
}

func benchmarkCreateTask(client pb.CoordinatorServiceClient, numTasks, concurrency int) BenchmarkResult {
	var (
		wg           sync.WaitGroup
		success      int64
		failed       int64
		totalLatency int64
	)

	taskChan := make(chan int, numTasks)
	for i := 0; i < numTasks; i++ {
		taskChan <- i
	}
	close(taskChan)

	priorities := []pb.Priority{
		pb.Priority_PRIORITY_LOW,
		pb.Priority_PRIORITY_NORMAL,
		pb.Priority_PRIORITY_HIGH,
		pb.Priority_PRIORITY_CRITICAL,
	}

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for taskNum := range taskChan {
				taskStart := time.Now()

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, err := client.CreateTask(ctx, &pb.CreateTaskRequest{
					Name:     fmt.Sprintf("bench-task-%d", taskNum),
					Command:  &pb.Command{Value: "echo hello"},
					Priority: priorities[taskNum%4],
				})
				cancel()

				elapsed := time.Since(taskStart).Microseconds()
				atomic.AddInt64(&totalLatency, elapsed)

				if err != nil {
					atomic.AddInt64(&failed, 1)
				} else {
					atomic.AddInt64(&success, 1)
				}
			}
		}()
	}

	wg.Wait()
	totalDuration := time.Since(start)

	return BenchmarkResult{
		TotalTasks:       numTasks,
		Concurrency:      concurrency,
		SuccessfulCreate: success,
		FailedCreate:     failed,
		TotalDuration:    totalDuration,
		CreateThroughput: float64(numTasks) / totalDuration.Seconds(),
		AvgCreateLatency: float64(totalLatency) / float64(success+failed),
	}
}

func benchmarkEndToEnd(client pb.CoordinatorServiceClient, numTasks, concurrency int, timeout time.Duration) E2EResult {
	result := E2EResult{TotalTasks: numTasks}

	// Phase 1: Create all tasks
	fmt.Printf("Phase 1: Creating %d tasks...\n", numTasks)
	createStart := time.Now()

	var wg sync.WaitGroup
	var mu sync.Mutex
	taskChan := make(chan int, numTasks)
	createdTaskIds := make([]int32, 0, numTasks)

	for i := 0; i < numTasks; i++ {
		taskChan <- i
	}
	close(taskChan)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for taskNum := range taskChan {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				resp, err := client.CreateTask(ctx, &pb.CreateTaskRequest{
					Name:    fmt.Sprintf("e2e-bench-%d", taskNum),
					Command: &pb.Command{Value: "echo benchmark"},
				})
				cancel()

				if err == nil && resp.Success {
					mu.Lock()
					createdTaskIds = append(createdTaskIds, resp.Id.Value)
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	result.CreateDuration = time.Since(createStart)
	result.TasksCreated = len(createdTaskIds)
	result.CreateThroughput = float64(result.TasksCreated) / result.CreateDuration.Seconds()

	fmt.Printf("  ✓ Created %d tasks in %v (%.2f tasks/sec)\n",
		result.TasksCreated, result.CreateDuration, result.CreateThroughput)

	if result.TasksCreated == 0 {
		fmt.Println("  ✗ No tasks created, cannot continue E2E benchmark")
		return result
	}

	// Phase 2: Wait for all tasks to complete
	fmt.Printf("\nPhase 2: Waiting for task completion (timeout: %v)...\n", timeout)
	completionStart := time.Now()

	// Build task ID list for batch query
	taskIdMsgs := make([]*pb.TaskId, len(createdTaskIds))
	for i, id := range createdTaskIds {
		taskIdMsgs[i] = &pb.TaskId{Value: id}
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeoutChan := time.After(timeout)

	lastPending := result.TasksCreated
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			resp, err := client.GetTaskStatusBatch(ctx, &pb.GetTaskStatusBatchRequest{
				TaskIds: taskIdMsgs,
			})
			cancel()

			if err != nil {
				fmt.Printf("  ⚠ Error polling status: %v\n", err)
				continue
			}

			completed := int(resp.Completed + resp.Failed)
			pending := int(resp.Pending + resp.Running)

			if pending != lastPending {
				fmt.Printf("  → Progress: %d/%d completed (pending: %d, running: %d)\n",
					completed, result.TasksCreated, resp.Pending, resp.Running)
				lastPending = pending
			}

			if pending == 0 {
				// All tasks done
				result.CompleteDuration = time.Since(completionStart)
				result.TotalE2EDuration = time.Since(createStart)
				result.TasksCompleted = int(resp.Completed)
				result.TasksFailed = int(resp.Failed)
				result.E2EThroughput = float64(completed) / result.TotalE2EDuration.Seconds()
				result.AvgE2ELatency = result.TotalE2EDuration / time.Duration(completed)

				fmt.Printf("  ✓ All tasks completed!\n")
				return result
			}

		case <-timeoutChan:
			// Timeout - get final status
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			resp, _ := client.GetTaskStatusBatch(ctx, &pb.GetTaskStatusBatchRequest{
				TaskIds: taskIdMsgs,
			})
			cancel()

			result.CompleteDuration = time.Since(completionStart)
			result.TotalE2EDuration = time.Since(createStart)
			if resp != nil {
				result.TasksCompleted = int(resp.Completed)
				result.TasksFailed = int(resp.Failed)
				completed := result.TasksCompleted + result.TasksFailed
				if completed > 0 {
					result.E2EThroughput = float64(completed) / result.TotalE2EDuration.Seconds()
					result.AvgE2ELatency = result.TotalE2EDuration / time.Duration(completed)
				}
			}

			fmt.Printf("  ⚠ Timeout reached! Some tasks may still be pending.\n")
			return result
		}
	}
}

func printE2EResults(r E2EResult) {
	fmt.Println("\n════════════════════════════════════════════════════════════")
	fmt.Println("                    E2E BENCHMARK RESULTS                    ")
	fmt.Println("════════════════════════════════════════════════════════════")

	fmt.Printf("\n  Task Summary:\n")
	fmt.Printf("   Total requested:    %d\n", r.TotalTasks)
	fmt.Printf("   Successfully created: %d\n", r.TasksCreated)
	fmt.Printf("   Completed (success): %d\n", r.TasksCompleted)
	fmt.Printf("   Completed (failed):  %d\n", r.TasksFailed)

	successRate := float64(r.TasksCompleted) / float64(r.TasksCreated) * 100
	fmt.Printf("   Success rate:       %.2f%%\n", successRate)

	fmt.Printf("\n ️ Timing:\n")
	fmt.Printf("   Task creation:      %v\n", r.CreateDuration)
	fmt.Printf("   Task completion:    %v\n", r.CompleteDuration)
	fmt.Printf("   Total E2E time:     %v\n", r.TotalE2EDuration)

	fmt.Printf("\n  Throughput:\n")
	fmt.Printf("   Create throughput:  %.2f tasks/sec\n", r.CreateThroughput)
	fmt.Printf("   E2E throughput:     %.2f tasks/sec\n", r.E2EThroughput)
	fmt.Printf("   Avg E2E latency:    %v/task\n", r.AvgE2ELatency)

	fmt.Println("\n════════════════════════════════════════════════════════════")
}

func printResults(r BenchmarkResult) {
	fmt.Printf("\nResults:\n")
	fmt.Printf("  Total tasks:        %d\n", r.TotalTasks)
	fmt.Printf("  Successful:         %d\n", r.SuccessfulCreate)
	fmt.Printf("  Failed:             %d\n", r.FailedCreate)
	fmt.Printf("  Total duration:     %v\n", r.TotalDuration)
	fmt.Printf("  Throughput:         %.2f tasks/sec\n", r.CreateThroughput)
	fmt.Printf("  Avg latency:        %.2f µs (%.2f ms)\n", r.AvgCreateLatency, r.AvgCreateLatency/1000)

	// Success rate
	successRate := float64(r.SuccessfulCreate) / float64(r.TotalTasks) * 100
	fmt.Printf("  Success rate:       %.2f%%\n", successRate)
}
