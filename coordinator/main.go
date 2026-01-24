package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
)

type Coordinator struct {
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	lis, err := net.Listen("tcp", "0.0.0.0:8000")
	if err != nil {
		log.Fatalf("Error in creating Coordinator server", err)
	}
	grpcServer := grpc.NewServer()
	grpcServer.Serve(lis)
}
