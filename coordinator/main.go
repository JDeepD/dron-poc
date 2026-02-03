package main

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	config "github.com/jdeepd/dron-poc/common"
	pb "github.com/jdeepd/dron-poc/dron_poc"
	"google.golang.org/grpc"
)

type Coordinator struct {
	pb.UnimplementedCoordinatorServiceServer
	my           sync.Mutex
	tasks        map[int32]*pb.Task
	workers      map[int32]*pb.WorkerID
	runningTasks map[int32]*pb.Task
}

func NewCoordinator() *Coordinator {
	return &Coordinator{
		tasks:        make(map[int32]*pb.Task),
		workers:      make(map[int32]*pb.WorkerID),
		runningTasks: make(map[int32]*pb.Task),
	}
}

func (c *Coordinator) StartWorkerMonitor(ctx context.Context) {
	ticker := time.NewTicker(config.HeartbeatInterval)
	// TODO
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	coordinator := NewCoordinator()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lis, err := net.Listen("tcp", "0.0.0.0:8000")
	if err != nil {
		log.Fatalf("Error in creating Coordinator server", err)
	}
	grpcServer := grpc.NewServer()
	grpcServer.Serve(lis)
}
