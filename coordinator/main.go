package main

import (
	"container/heap"
	"context"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jdeepd/dron-poc/common"
	pb "github.com/jdeepd/dron-poc/dron_poc"
	"google.golang.org/grpc"
)

type Coordinator struct {
	pb.UnimplementedCoordinatorServiceServer
	mu             sync.Mutex
	tasks          map[int32]*pb.Task
	workers        map[int32]*pb.Worker
	runningTasks   map[int32]*pb.Task   // taskId -> task
	completedTasks map[int32]*pb.Task   // taskId -> task
	taskQueue      common.PriorityQueue // pending tasks priority queue
	nextTaskId     int32
}

func NewCoordinator() *Coordinator {
	c := &Coordinator{
		tasks:          make(map[int32]*pb.Task),
		workers:        make(map[int32]*pb.Worker),
		runningTasks:   make(map[int32]*pb.Task),
		completedTasks: make(map[int32]*pb.Task),
		taskQueue:      make(common.PriorityQueue, 0),
		nextTaskId:     1,
	}
	heap.Init(&c.taskQueue)
	return c
}

func (c *Coordinator) StartWorkerMonitor(ctx context.Context) {
	ticker := time.NewTicker(common.HeartbeatInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				c.CheckWorkerStatus(ctx)
			}
		}
	}()
}

func (c *Coordinator) CheckWorkerStatus(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for id, worker := range c.workers {
		if worker.LastHeartbeat != nil {
			lastHB := worker.LastHeartbeat.AsTime()
			if now.Sub(lastHB) > common.WorkerTimeout {
				if worker.IsAlive {
					log.Warn().Int32("worker_id", id).Msgf("Worker %s timed out, marking as dead", worker.Name)
					worker.IsAlive = false
					// Re-queue the task if worker was working on one
					if worker.CurrentTask != nil {
						taskId := worker.CurrentTask.Value
						if task, exists := c.runningTasks[taskId]; exists {
							task.Status = pb.Status_STATUS_PENDING
							task.AssignedTo = nil
							heap.Push(&c.taskQueue, task)
							delete(c.runningTasks, taskId)
							log.Info().Int32("task_id", taskId).Msg("Re-queued task from dead worker")
						}
						worker.CurrentTask = nil
					}
				}
			}
		}
		if worker.IsAlive {
			log.Debug().Int32("worker_id", id).Msgf("Worker %s is alive", worker.Name)
		}
	}
}

func (c *Coordinator) RegisterWorker(ctx context.Context, req *pb.RegisterWorkerRequest) (*pb.RegisterWorkerResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	workerID := int32(len(c.workers) + 1)

	c.workers[workerID] = &pb.Worker{
		Id:            &pb.WorkerID{Value: workerID},
		Name:          req.Name,
		Address:       req.Address,
		IsAlive:       true,
		LastHeartbeat: timestamppb.Now(),
		CurrentTask:   nil,
	}

	log.Info().Int32("worker_id", workerID).Msgf("Worker %s registered successfully with ID %d", req.Name, workerID)

	return &pb.RegisterWorkerResponse{
		Success: true,
		Message: "Worker registered successfully",
		Id:      &pb.WorkerID{Value: workerID},
	}, nil
}

func (c *Coordinator) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.CreateTaskResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	taskId := c.nextTaskId
	c.nextTaskId++

	task := &pb.Task{
		Id:          &pb.TaskId{Value: taskId},
		Name:        req.Name,
		Command:     req.Command,
		Status:      pb.Status_STATUS_PENDING,
		AssignedTo:  nil,
		SubmittedAt: timestamppb.Now(),
		Priority:    pb.Priority_PRIORITY_NORMAL,
	}

	c.tasks[taskId] = task
	heap.Push(&c.taskQueue, task)

	log.Info().Int32("task_id", taskId).Str("name", req.Name).Msg("Task created successfully")

	return &pb.CreateTaskResponse{
		Id:      &pb.TaskId{Value: taskId},
		Success: true,
		Message: "Task created successfully",
	}, nil
}

func (c *Coordinator) AssignTask(ctx context.Context, req *pb.GetTaskRequest) (*pb.GetTaskResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	workerId := req.WorkerId.Value
	worker, exists := c.workers[workerId]
	if !exists {
		log.Warn().Int32("worker_id", workerId).Msgf("Worker with ID %d not found", workerId)
		return &pb.GetTaskResponse{
			HasTask: false,
			Message: "Worker not registered.",
		}, nil
	}

	// Update worker heartbeat
	worker.LastHeartbeat = timestamppb.Now()
	worker.IsAlive = true

	// Check if there are pending tasks
	if c.taskQueue.Len() == 0 {
		return &pb.GetTaskResponse{
			HasTask: false,
			Message: "No tasks available",
		}, nil
	}

	// Pop the highest priority task
	task := heap.Pop(&c.taskQueue).(*pb.Task)
	task.Status = pb.Status_STATUS_RUNNING
	task.AssignedTo = &pb.TaskId{Value: workerId}
	task.StartedAt = timestamppb.Now()

	// Update tracking maps
	c.runningTasks[task.Id.Value] = task
	worker.CurrentTask = task.Id

	log.Info().Int32("task_id", task.Id.Value).Int32("worker_id", workerId).
		Msgf("Task %s assigned to worker %s", task.Name, worker.Name)

	return &pb.GetTaskResponse{
		Task:    task,
		HasTask: true,
		Message: "Task assigned successfully",
	}, nil
}

func (c *Coordinator) FinishTask(ctx context.Context, req *pb.FinishTaskRequest) (*pb.FinishTaskResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	workerId := req.Id.Value
	taskId := req.TaskId.Value

	worker, workerExists := c.workers[workerId]
	if !workerExists {
		log.Warn().Int32("worker_id", workerId).Msg("Worker not found")
		return &pb.FinishTaskResponse{
			Success: false,
			Message: "Worker not registered",
		}, nil
	}

	// Update worker heartbeat
	worker.LastHeartbeat = timestamppb.Now()
	worker.IsAlive = true
	worker.CurrentTask = nil

	task, taskExists := c.runningTasks[taskId]
	if !taskExists {
		log.Warn().Int32("task_id", taskId).Msg("Task not found in running tasks")
		return &pb.FinishTaskResponse{
			Success: false,
			Message: "Task not found in running tasks",
		}, nil
	}

	// Update task status
	task.Status = req.Status
	task.CompletedAt = timestamppb.Now()

	// Move from running to completed
	delete(c.runningTasks, taskId)
	c.completedTasks[taskId] = task

	statusStr := "SUCCESS"
	if req.Status == pb.Status_STATUS_FAILURE {
		statusStr = "FAILURE"
	}

	log.Info().Int32("task_id", taskId).Int32("worker_id", workerId).
		Str("status", statusStr).
		Msgf("Task %s completed by worker %s with status %s", task.Name, worker.Name, statusStr)

	if req.Output != nil && req.Output.Value != "" {
		log.Info().Int32("task_id", taskId).Str("output", req.Output.Value).Msg("Task output")
	}

	return &pb.FinishTaskResponse{
		Success: true,
		Message: "Task completion recorded",
	}, nil
}

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	coordinator := NewCoordinator()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	coordinator.StartWorkerMonitor(ctx)

	lis, err := net.Listen("tcp", common.DefaultCoordinatorAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start coordinator server")
	}
	grpcServer := grpc.NewServer()
	pb.RegisterCoordinatorServiceServer(grpcServer, coordinator)
	log.Info().Msgf("Coordinator server is listening on %s", common.DefaultCoordinatorAddress)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msgf("Failed to serve coordinator server: %v", err)
	}
}
