package main

import (
	"context"
	"flag"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	config "github.com/jdeepd/dron-poc/common"
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
	log.Info().Int32("worker_id", w.id).Msgf("Worker %s registered successfully with ID %d", w.name, w.id)
	return nil
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	name := flag.String("name", "", "Name of the worker")
	coordinatorAddress := flag.String("coordinator", config.DefaultCoordinatorAddress, "Address of the coordinator")
	address := flag.String("address", "", "Address of the worker")

	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if *name == "" {
		log.Error().Msg("Worker name is required")
	}
	if *address == "" {
		log.Error().Msg("Worker address is required")
	}
	if *coordinatorAddress == "" {
		log.Error().Msg("Coordinator address is required")
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
}
