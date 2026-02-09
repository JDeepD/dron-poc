package main

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	config "github.com/jdeepd/dron-poc/common"
	pb "github.com/jdeepd/dron-poc/dron_poc"
	"google.golang.org/grpc"
)

type Coordinator struct {
	pb.UnimplementedCoordinatorServiceServer
	my           sync.Mutex
	tasks        map[int32]*pb.Task
	workers      map[int32]*pb.Worker
	runningTasks map[int32]*pb.Task
}

func NewCoordinator() *Coordinator {
	return &Coordinator{
		tasks:        make(map[int32]*pb.Task),
		workers:      make(map[int32]*pb.Worker),
		runningTasks: make(map[int32]*pb.Task),
	}
}

func (c *Coordinator) StartWorkerMonitor(ctx context.Context) {
	ticker := time.NewTicker(config.HeartbeatInterval)
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
	// TODO
}

func (c *Coordinator) CheckWorkerStatus(ctx context.Context) {
	for workerID := range c.workers {
		log.Info().Msgf("Tick: Checking worker status for worker ID %d", workerID)
	}
}

func (c *Coordinator) RegisterWorker(ctx context.Context, req *pb.RegisterWorkerRequest) (*pb.RegisterWorkerResponse, error) {
	c.my.Lock()
	defer c.my.Unlock()

	workerID := int32(len(c.workers) + 1)

	c.workers[workerID] = &pb.Worker{
		Id:            &pb.WorkerID{Value: workerID},
		Name:          req.Name,
		Address:       req.Address,
		IsAlive:       true,
		LastHeartbeat: nil,
		CurrentTask:   nil,
	}

	log.Info().Int32("worker_id", workerID).Msgf("Worker %s registered successfully with ID %d", req.Name, workerID)

	return &pb.RegisterWorkerResponse{
		Success: true,
		Message: "Worker registered successfully",
		Id:      &pb.WorkerID{Value: workerID},
	}, nil
}

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	coordinator := NewCoordinator()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	coordinator.StartWorkerMonitor(ctx)

	lis, err := net.Listen("tcp", config.DefaultCoordinatorAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start coordinator server")
	}
	grpcServer := grpc.NewServer()
	pb.RegisterCoordinatorServiceServer(grpcServer, coordinator)
	log.Info().Msgf("Coordinator server is listening on %s", config.DefaultCoordinatorAddress)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msgf("Failed to serve coordinator server: %v", err)
	}
}
