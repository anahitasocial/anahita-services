package providers

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

// WHAT THIS GUARDS (roadmap row 96, part 5): a server that takes the
// connection and then says nothing does not hold a send over TLS for good;
// it gives up when the send's time is up, as a plain connection does.
func TestATLSServerThatStallsIsGivenUpOn(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close() // held open, silent
		}
	}()

	host, port, _ := net.SplitHostPort(listener.Addr().String())
	portNumber, _ := strconv.Atoi(port)
	mailer := &SmtpMailer{host: host, port: portNumber, secure: true}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := mailer.dial(ctx)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("a silent server was taken for a mail server")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("still waiting on a silent server after its time was up")
	}
}
