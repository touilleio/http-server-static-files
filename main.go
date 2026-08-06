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
	"strings"
	"syscall"
	"time"
)

type envConfig struct {
	Port                    string
	RootPath                string
	ReadHeaderTimeout       time.Duration
	ReadTimeout             time.Duration
	WriteTimeout            time.Duration
	IdleTimeout             time.Duration
	ContentSecurityPolicy   string
	ReferrerPolicy          string
	PermissionsPolicy       string
	FrameOptions            string
	StrictTransportSecurity string
}

func main() {
	// Log the application startup message with version, commit, build date, and OS architecture.
	log.Println("http-server-static-files application is starting...")
	log.Printf("Version    : %s", Version)
	log.Printf("Commit     : %s", GitCommit)
	log.Printf("Build date : %s", BuildDate)
	log.Printf("OSarch     : %s", OsArch)

	// Define the environment configuration variables and use defaults if not provided.
	env, err := loadEnvConfig()
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	root, err := os.OpenRoot(env.RootPath)
	if err != nil {
		log.Fatalf("Failed to open static file root %q: %v", env.RootPath, err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			log.Printf("Failed to close static file root: %v", err)
		}
	}()

	fileHandler := http.FileServerFS(root.FS())
	handler := securityHeaders(env, allowedMethods(fileHandler))

	// Create a context that is canceled by shutdown signals (e.g., SIGINT, SIGTERM).
	shutdownSignal, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start the HTTP server with bounded request and connection lifetimes.
	s := &http.Server{
		Addr:              fmt.Sprint(":", env.Port),
		Handler:           handler,
		ReadHeaderTimeout: env.ReadHeaderTimeout,
		ReadTimeout:       env.ReadTimeout,
		WriteTimeout:      env.WriteTimeout,
		IdleTimeout:       env.IdleTimeout,
	}
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

func loadEnvConfig() (envConfig, error) {
	readHeaderTimeout, err := durationFromEnv("READ_HEADER_TIMEOUT", "1s")
	if err != nil {
		return envConfig{}, err
	}
	readTimeout, err := durationFromEnv("READ_TIMEOUT", "1s")
	if err != nil {
		return envConfig{}, err
	}
	writeTimeout, err := durationFromEnv("WRITE_TIMEOUT", "1s")
	if err != nil {
		return envConfig{}, err
	}
	idleTimeout, err := durationFromEnv("IDLE_TIMEOUT", "1s")
	if err != nil {
		return envConfig{}, err
	}

	config := envConfig{
		Port:                    envOrDefault("PORT", "8080"),
		RootPath:                envOrDefault("ROOT_PATH", "/static"),
		ReadHeaderTimeout:       readHeaderTimeout,
		ReadTimeout:             readTimeout,
		WriteTimeout:            writeTimeout,
		IdleTimeout:             idleTimeout,
		ContentSecurityPolicy:   envOrDefault("CONTENT_SECURITY_POLICY", "default-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'"),
		ReferrerPolicy:          envOrDefault("REFERRER_POLICY", "strict-origin-when-cross-origin"),
		PermissionsPolicy:       envOrDefault("PERMISSIONS_POLICY", "accelerometer=(), camera=(), geolocation=(), gyroscope=(), microphone=(), payment=(), usb=()"),
		FrameOptions:            envOrDefault("FRAME_OPTIONS", "DENY"),
		StrictTransportSecurity: os.Getenv("STRICT_TRANSPORT_SECURITY"),
	}

	for name, value := range map[string]string{
		"CONTENT_SECURITY_POLICY":   config.ContentSecurityPolicy,
		"REFERRER_POLICY":           config.ReferrerPolicy,
		"PERMISSIONS_POLICY":        config.PermissionsPolicy,
		"FRAME_OPTIONS":             config.FrameOptions,
		"STRICT_TRANSPORT_SECURITY": config.StrictTransportSecurity,
	} {
		if strings.ContainsAny(value, "\r\n") {
			return envConfig{}, fmt.Errorf("%s must not contain line breaks", name)
		}
	}

	return config, nil
}

func durationFromEnv(key, defaultValue string) (time.Duration, error) {
	value := envOrDefault(key, defaultValue)
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}

	return duration, nil
}

func allowedMethods(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func securityHeaders(config envConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		setHeaderIfConfigured(w, "Content-Security-Policy", config.ContentSecurityPolicy)
		setHeaderIfConfigured(w, "Referrer-Policy", config.ReferrerPolicy)
		setHeaderIfConfigured(w, "Permissions-Policy", config.PermissionsPolicy)
		setHeaderIfConfigured(w, "X-Frame-Options", config.FrameOptions)
		setHeaderIfConfigured(w, "Strict-Transport-Security", config.StrictTransportSecurity)
		next.ServeHTTP(w, r)
	})
}

func setHeaderIfConfigured(w http.ResponseWriter, name, value string) {
	if value != "" {
		w.Header().Set(name, value)
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
