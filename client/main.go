package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/jdeepd/dron-poc/common"
	pb "github.com/jdeepd/dron-poc/dron_poc"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Subcommands
	createCmd := flag.NewFlagSet("create", flag.ExitOnError)
	createName := createCmd.String("name", "", "Name of the task")
	createCommand := createCmd.String("command", "", "Command to execute")
	createPriority := createCmd.String("priority", "normal", "Task priority: low, normal, high, critical")
	createCoordinator := createCmd.String("coordinator", common.DefaultCoordinatorAddress, "Coordinator address")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "create":
		createCmd.Parse(os.Args[2:])
		if *createName == "" || *createCommand == "" {
			log.Fatal().Msg("Both -name and -command are required for create")
		}
		createTask(*createCoordinator, *createName, *createCommand, *createPriority)
	case "help":
		printUsage()
	default:
		log.Error().Str("command", os.Args[1]).Msg("Unknown command")
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	log.Info().Msg("Usage: client <command> [options]")
	log.Info().Msg("")
	log.Info().Msg("Commands:")
	log.Info().Msg("  create   Create a new task")
	log.Info().Msg("    -name        Name of the task")
	log.Info().Msg("    -command     Command to execute")
	log.Info().Msg("    -priority    Task priority: low, normal, high, critical (default: normal)")
	log.Info().Msg("    -coordinator Coordinator address (default: 0.0.0.0:8000)")
	log.Info().Msg("")
	log.Info().Msg("  help     Show this help message")
}

func parsePriority(p string) pb.Priority {
	switch p {
	case "low":
		return pb.Priority_PRIORITY_LOW
	case "normal":
		return pb.Priority_PRIORITY_NORMAL
	case "high":
		return pb.Priority_PRIORITY_HIGH
	case "critical":
		return pb.Priority_PRIORITY_CRITICAL
	default:
		log.Warn().Str("priority", p).Msg("Unknown priority, using normal")
		return pb.Priority_PRIORITY_NORMAL
	}
}

func createTask(coordinatorAddr, name, command, priority string) {
	conn, err := grpc.NewClient(coordinatorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to coordinator")
	}
	defer conn.Close()

	client := pb.NewCoordinatorServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.CreateTaskRequest{
		Name:     name,
		Command:  &pb.Command{Value: command},
		Priority: parsePriority(priority),
	}

	resp, err := client.CreateTask(ctx, req)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create task")
	}

	if !resp.Success {
		log.Error().Str("message", resp.Message).Msg("Failed to create task")
		return
	}

	log.Info().Int32("task_id", resp.Id.Value).
		Str("name", name).
		Str("command", command).
		Msg("Task created successfully")
}
