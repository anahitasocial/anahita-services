package main

import (
	"file/cmd/server/handlers"
	"file/data/services"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	"anahita-libs/proto/filepb"
	"file/config"
)

const (
	gRpcPort = "50051"
)

func main() {
	// if we crash the go code, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// create config
	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	hEnv := &handlers.Env{
		Config:   config,
		Services: services.NewServices(config),
	}
	h := handlers.NewHandlers(hEnv)

	// start the gRPC server
	if err := startGrpcServer(h); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func startGrpcServer(h *handlers.Handlers) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", gRpcPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()

	// register the service
	filepb.RegisterFileServiceServer(s, h.File)

	log.Printf("gRPC server started at port %s", gRpcPort)

	if err := s.Serve(lis); err != nil {
		return err
	}

	return nil
}
