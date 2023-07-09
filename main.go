package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Route is an http.Handler that knows the mux pattern
// under which it will be registered.
type Route interface {
	http.Handler

	// Pattern reports the path at which this is registered.
	Pattern() string
}

type DummyStruct struct{}

func (*DummyStruct) DoNothing() {
	fmt.Println("--- DoNothing called")
}

func NewDummyStruct() *DummyStruct {
	fmt.Println("--- NewDummyStruct called ---")
	return &DummyStruct{}
}

// EchoHandler is an http.Handler that copies its request body
// back to the response.
type EchoHandler struct {
	log *zap.Logger
}

// ServeHTTP handles an HTTP request to the /echo endpoint.
func (h *EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("--- ServeHTTP of EchoHandler called ---")
	if _, err := io.Copy(w, r.Body); err != nil {
		h.log.Warn("Failed to handle request:", zap.Error(err))
	}
}

func (*EchoHandler) Pattern() string {
	return "/echo"
}

// NewEchoHandler builds a new EchoHandler.
func NewEchoHandler(log *zap.Logger) *EchoHandler {
	fmt.Println("--- NewEchoHandler called ---")
	return &EchoHandler{log: log}
}

func NewHTTPServer(lc fx.Lifecycle, mux *http.ServeMux, log *zap.Logger) *http.Server {
	fmt.Println("--- NewHTTPServer called ---")
	srv := &http.Server{Addr: ":8080", Handler: mux}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}
			log.Info("Starting HTTP server", zap.String("addr", srv.Addr))
			go srv.Serve(ln)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
	return srv
}

// NewServeMux builds a ServeMux that will route requests
// to the given EchoHandler.
func NewServeMux(route Route) *http.ServeMux {
	fmt.Println("--- NewServeMux called ---")
	mux := http.NewServeMux()
	mux.Handle(route.Pattern(), route)
	return mux
}

func main() {
	fx.New(
		fx.Provide(
			NewHTTPServer,
			fx.Annotate(
				NewEchoHandler,
				fx.As(new(Route)),
			),
			NewServeMux,
			NewDummyStruct, // just for experimenting
			zap.NewExample,
		),
		// *DummyStruct added to the args so NewDummyStruct will be called inorder to get the dependency of *DummyStruct
		fx.Invoke(func(*DummyStruct, *http.Server) {
			// Since *http.Server is served by the NewHTTPServer function
			// NewHTTPServer is executed to return *http.Server (fullfilling dependency)
			// Since NewHTTPServer is dependent upon *http.ServeMux
			// NewServeMux will be executed to return *http.ServeMux
			// NewServeMux is dependent upon *EchoHandler
			// NewEchoHandler will be executed to return *EchoHandler
			// Since *EchoHandler is the ground level (the last underneath dependency required) dependency
			// The Execution will be
			// NewDummyStruct will be called to provided *DummyStruct first
			// And to provide *http.Server, NewEchoHandler -> NewServeMux -> NewHTTPServer in respective order will be executed
		}),
	).Run()
}
