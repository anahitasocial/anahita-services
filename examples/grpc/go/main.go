// Reads one node from Anahita's graph service over gRPC, with mutual TLS.
//
//	buf generate ../../../anahita-libs --path ../../../anahita-libs/proto/graphpb
//	go run . 42
//
// Inside a checkout of anahita-services, whose go.work does not list this
// example, run it with GOWORK=off.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"example.com/anahita-grpc/gen/proto/graphpb"
)

func main() {
	id, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil {
		log.Fatalf("a node's id, a number: %v", err)
	}
	tlsDir := env("GRPC_TLS_DIR", "/run/secrets/grpc")
	address := env("GRAPH_SERVICE_ADDR", "graph-grpc-service:50051")

	// This service's certificate, and the cluster's authority, which signed
	// it and every Anahita service's.
	cert, err := tls.LoadX509KeyPair(filepath.Join(tlsDir, "tls.crt"), filepath.Join(tlsDir, "tls.key"))
	if err != nil {
		log.Fatal(err)
	}
	ca, err := os.ReadFile(filepath.Join(tlsDir, "ca.crt"))
	if err != nil {
		log.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(ca)
	creds := credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	})

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reply, err := graphpb.NewNodeServiceClient(conn).Read(ctx, &graphpb.NodeReadRequest{Id: id})
	if err != nil {
		log.Fatal(err)
	}

	node := reply.GetNode()
	fmt.Println(node.GetId(), node.GetType(), "@"+node.GetAlias(), node.GetName())
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
