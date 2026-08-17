package main

import (
	"fmt"
	"log"
	"net"

	"anahita-libs/proto/mailpb"

	"mail/cmd/server/handlers"
	"mail/config"
	"mail/providers"

	"google.golang.org/grpc"
)

const (
	gRpcPort = "50056"
)

func main() {
	// if we crash the go code, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// create config
	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	provider, err := providers.NewProvider(config)
	if err != nil {
		log.Fatalf("Failed to initialize provider: %v", err)
	}

	hEnv := &handlers.Env{
		Config:   config,
		Provider: provider,
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
	mailpb.RegisterMailServiceServer(s, h.Mail)

	log.Printf("gRPC server started at port %s", gRpcPort)

	if err := s.Serve(lis); err != nil {
		return err
	}

	return nil
}
