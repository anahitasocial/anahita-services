package main

import (
	"anahita-libs/grpctls"
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

	srv, err := services.NewServices(config)
	if err != nil {
		log.Fatalf("storage setup failed: %v", err)
	}

	hEnv := &handlers.Env{
		Config:   config,
		Services: srv,
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

	// Mutual TLS: only callers holding a certificate from this installation's
	// authority are served. Refuses to start without certificates unless
	// GRPC_TLS says otherwise.
	transport, err := grpctls.ServerOption()
	if err != nil {
		return err
	}

	s := grpc.NewServer(transport)

	// register the service
	filepb.RegisterFileServiceServer(s, h.File)

	log.Printf("gRPC server started at port %s", gRpcPort)

	if err := s.Serve(lis); err != nil {
		return err
	}

	return nil
}
