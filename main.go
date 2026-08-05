package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

type envConfig struct {
	Port     string
	RootPath string
}

func main() {
	// Log the application startup message with version, commit, build date, and OS architecture.
	log.Println("http-server-static-files application is starting...")
	log.Printf("Version    : %s", Version)
	log.Printf("Commit     : %s", GitCommit)
	log.Printf("Build date : %s", BuildDate)
	log.Printf("OSarch     : %s", OsArch)

	// Define the environment configuration variables and use defaults if not provided.
	env := envConfig{
		Port:     envOrDefault("PORT", "8080"),
		RootPath: envOrDefault("ROOT_PATH", "/static"),
	}

	// Create a context that is canceled by shutdown signals (e.g., SIGINT, SIGTERM).
	shutdownSignal, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Set up the HTTP server to serve static files from the specified root path.
	http.Handle("/", http.FileServer(http.Dir(env.RootPath)))

	// Start the HTTP server on the configured port.
	s := &http.Server{Addr: fmt.Sprint(":", env.Port)}
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- s.ListenAndServe()
	}()

	// Log the information about which directory is being served as static files.
	log.Printf("Now serving static files in %s", env.RootPath)

	// Block until a shutdown signal is received or the server stops unexpectedly.
	select {
	case <-shutdownSignal.Done():
		log.Printf("Shutdown signal received, exiting...")
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server failed: %v", err)
		}
		return
	}

	// Give active requests a bounded window to finish before exiting.
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Shutdown(shutdownContext); err != nil {
		log.Fatalf("Got an error while shutting down: %v\n", err)
	}

	if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("HTTP server failed while shutting down: %v", err)
	}
}

func envOrDefault(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return defaultValue
}

// GitCommit the git commit that was compiled. This will be filled in by the compiler.
var GitCommit string

// Version the main version number that is being run at the moment.
var Version = "X.Y.Z"

// BuildDate datetime that binary was created
var BuildDate = ""

// GoVersion go runtime version
var GoVersion = runtime.Version()

// OsArch OS architecture
var OsArch = fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH)
