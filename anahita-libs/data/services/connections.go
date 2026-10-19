package services

import (
	"sync"

	"anahita-libs/grpctls"

	"google.golang.org/grpc"
)

// connections holds one gRPC connection for each address, made the first
// time it is asked for and kept for as long as the process runs. A
// connection is safe to share and reconnects by itself, so there is no need
// for more than one; a client that made one for every call left each behind,
// open, and paid a new handshake each time (roadmap row 96, part 5).
var connections = struct {
	sync.Mutex
	byAddress map[string]*grpc.ClientConn
}{byAddress: map[string]*grpc.ClientConn{}}

// sharedConnection is the connection to an address, made once.
func sharedConnection(address string) (*grpc.ClientConn, error) {
	connections.Lock()
	defer connections.Unlock()

	if conn, ok := connections.byAddress[address]; ok {
		return conn, nil
	}

	transport, err := grpctls.DialOption()
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(address,
		transport,
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
	)
	if err != nil {
		return nil, err
	}
	connections.byAddress[address] = conn
	return conn, nil
}
