package main

import (
	"context"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
)

type envConfig struct {
	Port     string `envconfig:"PORT" default:"8080"`
	RootPath string `envconfig:"ROOT_PATH" default:"/static"`
}

func main() {
	// Log the application startup message with version, commit, build date, and OS architecture.
	log.Println("http-server-static-files application is starting...")
	log.Printf("Version    : %s", Version)
	log.Printf("Commit     : %s", GitCommit)
	log.Printf("Build date : %s", BuildDate)
	log.Printf("OSarch     : %s", OsArch)

	// Define the environment configuration variables and process them from the environment or use defaults if not provided.
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Printf("[ERROR] Failed to process env var: %s\n", err)
		return
	}

	// Create a channel to listen for shutdown signals (e.g., SIGINT, SIGTERM).
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	var wg sync.WaitGroup

	// Set up the HTTP server to serve static files from the specified root path.
	http.Handle("/", http.FileServer(http.Dir(env.RootPath)))

	// Start the HTTP server on the configured port.
	s := &http.Server{Addr: fmt.Sprint(":", env.Port)}
	go func() {
		log.Fatal(s.ListenAndServe())
	}()

	// Log the information about which directory is being served as static files.
	log.Printf("Now serving static files in %s", env.RootPath)

	// Block until a shutdown signal is received.
	<-signalChan
	log.Printf("Shutdown signal received, exiting...")

	// Gracefully shut down the HTTP server with a context background and log any errors during shutdown.
	err := s.Shutdown(context.Background())
	if err != nil {
		log.Fatalf("Got an error while shutting down: %v\n", err)
	}

	// Wait for all processing to complete properly before exiting the application.
	wg.Wait()
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
