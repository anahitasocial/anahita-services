package main

import (
	"anahita-libs/grpctls"
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
	// 50051, matching what every consumer is configured to dial:
	// MAIL_SERVICE_ADDR is "mail-grpc-service:50051" in docker-compose.yml and
	// the k8s service targets 50051 too. This listened on 50056, so nothing was
	// accepting connections on the port the rest of the stack used — and since
	// every notifier is fire-and-forget, no email has been delivered and no
	// error has ever surfaced. The compose file publishes 50056 on the host,
	// which is why the mismatch looked plausible.
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

	// Mutual TLS: only callers holding a certificate from this installation's
	// authority are served. Refuses to start without certificates unless
	// GRPC_TLS says otherwise.
	transport, err := grpctls.ServerOption()
	if err != nil {
		return err
	}

	s := grpc.NewServer(transport)

	// register the service
	mailpb.RegisterMailServiceServer(s, h.Mail)

	log.Printf("gRPC server started at port %s", gRpcPort)

	if err := s.Serve(lis); err != nil {
		return err
	}

	return nil
}
