// Package server is how a service starts and stops (roadmap row 170: each
// service's main.go kept its own copy, about 700 lines in all, and none
// stopped gracefully).
//
// Stopping: Kubernetes sends SIGTERM when it moves or replaces a pod, and
// kills it 30 seconds later unless told otherwise. A service that simply
// died on SIGTERM dropped whatever it was answering. Run and ServeGRPC
// stop taking new requests and let those already taken finish, for up to
// ShutdownWait, which is within those 30 seconds.
package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	an_mw "anahita-libs/middleware"
	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc"
)

// Port is where an API service is reached, behind the gateway.
const Port = "80"

// ShutdownWait is how long requests in flight are given to finish when a
// service is told to stop.
const ShutdownWait = 20 * time.Second

// New is the HTTP server an API service runs: the visitor's address read
// as the gateway gives it (ClientIPExtractor), each request logged, a panic
// answered 500 rather than ending the process, a body no larger than
// bodyLimit ("2M"; empty for none here, where a route sets its own),
// requests checked by their validate tags, and /ping, which answers pong.
func New(dev bool, bodyLimit string) *echo.Echo {
	e := echo.New()
	e.IPExtractor = an_mw.ClientIPExtractor()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	if bodyLimit != "" {
		e.Use(middleware.BodyLimit(bodyLimit))
	}
	e.Validator = validators.NewRequestValidator(validator.New())
	e.Debug = dev
	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})
	return e
}

// signInConfig is what the sign-in middleware reads: Valkey, for sessions,
// and where auth-service publishes the keys tokens are checked with.
type signInConfig interface {
	an_mw.ConfigInterface
	GetOAuthJWKSURL() string
}

// SignIn is the middleware for a service's routes: authorize refuses a
// request with no valid token, and setViewer says who asks when a token
// is there and lets anybody through when it is not. Handlers read who it
// is with an_mw.GetViewer. The keys are fetched from auth-service once and
// kept, so a token is checked here, with no call.
func SignIn(config signInConfig) (authorize, setViewer echo.MiddlewareFunc) {
	jwks := an_mw.NewJWKSClient(config.GetOAuthJWKSURL())
	return an_mw.Authorize(config, jwks), an_mw.SetViewer(config, jwks)
}

// stopping is a context that ends when the process is told to stop.
func stopping() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
}

// Run serves e on Port until the process is told to stop, then stops it
// gracefully. It ends the process if the server cannot start.
func Run(e *echo.Echo) {
	run(e, fmt.Sprintf(":%s", Port))
}

func run(e *echo.Echo, address string) {
	ctx, stop := stopping()
	defer stop()

	failed := make(chan error, 1)
	go func() {
		if err := e.Start(address); err != nil && !errors.Is(err, http.ErrServerClosed) {
			failed <- err
		}
	}()

	select {
	case err := <-failed:
		log.Fatal(err)
	case <-ctx.Done():
	}

	log.Printf("stopping: finishing what is in flight, for up to %s", ShutdownWait)
	wait, cancel := context.WithTimeout(context.Background(), ShutdownWait)
	defer cancel()
	if err := e.Shutdown(wait); err != nil {
		log.Printf("stopping: %v", err)
	}
}

// ServeGRPC serves s on lis until the process is told to stop, then stops
// it gracefully: the calls in flight finish, for up to ShutdownWait, and
// any left then are ended. It ends the process if s cannot serve.
func ServeGRPC(s *grpc.Server, lis net.Listener) {
	ctx, stop := stopping()
	defer stop()

	failed := make(chan error, 1)
	go func() {
		if err := s.Serve(lis); err != nil {
			failed <- err
		}
	}()

	select {
	case err := <-failed:
		log.Fatal(err)
	case <-ctx.Done():
	}

	log.Printf("stopping: finishing the calls in flight, for up to %s", ShutdownWait)
	done := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(ShutdownWait):
		s.Stop()
	}
}
