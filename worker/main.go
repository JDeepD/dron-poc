package main

import (
	"context"
	"flag"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/jdeepd/dron-poc/common"
	pb "github.com/jdeepd/dron-poc/dron_poc"
)

type Worker struct {
	id                int32
	name              string
	address           string
	coordinatorAddr   string
	coordinatorClient pb.CoordinatorServiceClient
	conn              *grpc.ClientConn

	mu          sync.Mutex
	currentTask *pb.Task
	isBusy      bool
}

func NewWorker(name, address, coordinatorAddr string) *Worker {
	return &Worker{
		name:            name,
		address:         address,
		coordinatorAddr: coordinatorAddr,
	}
}

func (w *Worker) Connect() error {
	conn, err := grpc.NewClient(w.coordinatorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to coordinator")
		return err
	}
	w.conn = conn
	w.coordinatorClient = pb.NewCoordinatorServiceClient(conn)
	return nil
}

func (w *Worker) Close() {
	if w.conn != nil {
		w.conn.Close()
	}
}

func (w *Worker) Register(ctx context.Context) error {
	req := &pb.RegisterWorkerRequest{
		Name:    w.name,
		Address: &pb.Address{Value: w.address},
	}
	resp, err := w.coordinatorClient.RegisterWorker(ctx, req)

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to register worker")
		return err
	}
	if !resp.Success {
		log.Fatal().Str("message: ", resp.Message).Msgf("Failed to register worker: %s", w.name)
		return nil
	}

	w.id = resp.Id.Value
	log.Debug().Int32("worker_id", w.id).Msgf("Worker %s registered successfully with ID %d", w.name, w.id)
	return nil
}

func (w *Worker) Poll(ctx context.Context) {
	ticker := time.NewTicker(common.WorkerPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.getTask(ctx)
		case <-ctx.Done():
			log.Debug().Msgf("Worker %s is shutting down", w.name)
			return
		}
	}
}

func (w *Worker) getTask(ctx context.Context) {
	w.mu.Lock()
	if w.isBusy {
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()

	req := &pb.GetTaskRequest{
		WorkerId: &pb.WorkerID{Value: w.id},
	}

	resp, err := w.coordinatorClient.AssignTask(ctx, req)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get task for worker %s", w.name)
		return
	}

	if !resp.HasTask {
		log.Debug().Msgf("No task assigned to worker %s", w.name)
		return
	}

	// Execute the task
	w.mu.Lock()
	w.currentTask = resp.Task
	w.isBusy = true
	w.mu.Unlock()

	log.Debug().Int32("task_id", resp.Task.Id.Value).
		Str("task_name", resp.Task.Name).
		Msgf("Worker %s received task: %s", w.name, resp.Task.Name)

	// Execute in a goroutine so we can continue polling (heartbeat)
	go w.executeTask(ctx, resp.Task)
}

func (w *Worker) executeTask(ctx context.Context, task *pb.Task) {
	defer func() {
		w.mu.Lock()
		w.currentTask = nil
		w.isBusy = false
		w.mu.Unlock()
	}()

	command := task.Command.Value
	log.Debug().Int32("task_id", task.Id.Value).
		Str("command", command).
		Msgf("Executing task %s: %s", task.Name, command)

	// Parse command and execute
	parts := strings.Fields(command)
	if len(parts) == 0 {
		w.reportTaskCompletion(ctx, task.Id, pb.Status_STATUS_FAILURE, "Empty command")
		return
	}

	var cmd *exec.Cmd
	if len(parts) == 1 {
		cmd = exec.CommandContext(ctx, parts[0])
	} else {
		cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
	}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		log.Error().Err(err).Int32("task_id", task.Id.Value).
			Str("output", outputStr).
			Msgf("Task %s failed", task.Name)
		w.reportTaskCompletion(ctx, task.Id, pb.Status_STATUS_FAILURE, outputStr)
		return
	}

	log.Debug().Int32("task_id", task.Id.Value).
		Str("output", outputStr).
		Msgf("Task %s completed successfully", task.Name)
	w.reportTaskCompletion(ctx, task.Id, pb.Status_STATUS_SUCCESS, outputStr)
}

func (w *Worker) reportTaskCompletion(ctx context.Context, taskId *pb.TaskId, status pb.Status, output string) {
	req := &pb.FinishTaskRequest{
		Id:     &pb.WorkerID{Value: w.id},
		TaskId: taskId,
		Status: status,
		Output: &pb.TaskOutput{Value: output},
	}

	resp, err := w.coordinatorClient.FinishTask(ctx, req)
	if err != nil {
		log.Error().Err(err).Int32("task_id", taskId.Value).
			Msg("Failed to report task completion to coordinator")
		return
	}

	if !resp.Success {
		log.Warn().Int32("task_id", taskId.Value).
			Str("message", resp.Message).
			Msg("Coordinator rejected task completion")
		return
	}

	log.Debug().Int32("task_id", taskId.Value).Msg("Task completion reported to coordinator")
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	name := flag.String("name", "", "Name of the worker")
	coordinatorAddress := flag.String("coordinator", common.DefaultCoordinatorAddress, "Address of the coordinator")
	address := flag.String("address", "", "Address of the worker")

	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if *name == "" {
		log.Fatal().Msg("Worker name is required")
	}
	if *address == "" {
		log.Fatal().Msg("Worker address is required")
	}
	if *coordinatorAddress == "" {
		log.Fatal().Msg("Coordinator address is required")
	}

	worker := NewWorker(*name, *address, *coordinatorAddress)
	if err := worker.Connect(); err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to coordinator")
	}
	defer worker.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := worker.Register(ctx); err != nil {
		log.Fatal().Err(err).Msgf("Failed to register worker: %s", worker.name)
	}

	// Start polling for tasks
	worker.Poll(ctx)
}
