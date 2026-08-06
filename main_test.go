package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDurationFromEnv(t *testing.T) {
	t.Setenv("TEST_TIMEOUT", "250ms")

	got, err := durationFromEnv("TEST_TIMEOUT", "1s")
	if err != nil {
		t.Fatalf("durationFromEnv returned an error: %v", err)
	}
	if got != 250*time.Millisecond {
		t.Fatalf("durationFromEnv = %v, want %v", got, 250*time.Millisecond)
	}
}

func TestDurationFromEnvDefault(t *testing.T) {
	const key = "HTTP_SERVER_STATIC_FILES_TEST_DEFAULT_TIMEOUT"
	oldValue, wasSet := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv(key, oldValue)
		} else {
			_ = os.Unsetenv(key)
		}
	})

	got, err := durationFromEnv(key, "1s")
	if err != nil {
		t.Fatalf("durationFromEnv returned an error: %v", err)
	}
	if got != time.Second {
		t.Fatalf("durationFromEnv = %v, want %v", got, time.Second)
	}
}

func TestDurationFromEnvRejectsNonPositiveValues(t *testing.T) {
	t.Setenv("TEST_TIMEOUT", "0s")

	if _, err := durationFromEnv("TEST_TIMEOUT", "1s"); err == nil {
		t.Fatal("durationFromEnv accepted a zero duration")
	}
}

func TestAllowedMethods(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := allowedMethods(next)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(method, "/", nil))

			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
			}
		})
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
	if got := response.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow header = %q, want %q", got, "GET, HEAD")
	}
}

func TestSecurityHeaders(t *testing.T) {
	config := envConfig{
		ContentSecurityPolicy:   "default-src 'self'",
		ReferrerPolicy:          "no-referrer",
		PermissionsPolicy:       "camera=()",
		FrameOptions:            "DENY",
		StrictTransportSecurity: "max-age=31536000",
	}
	handler := securityHeaders(config, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	want := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"Content-Security-Policy":   config.ContentSecurityPolicy,
		"Referrer-Policy":           config.ReferrerPolicy,
		"Permissions-Policy":        config.PermissionsPolicy,
		"X-Frame-Options":           config.FrameOptions,
		"Strict-Transport-Security": config.StrictTransportSecurity,
	}
	for name, value := range want {
		if got := response.Header().Get(name); got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
}

func TestOpenRootFileServer(t *testing.T) {
	rootDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDirectory, "index.html"), []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}

	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })

	response := httptest.NewRecorder()
	http.FileServerFS(root.FS()).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Body.String() != "safe" {
		t.Fatalf("body = %q, want %q", response.Body.String(), "safe")
	}
}

func TestOpenRootPreventsSymlinkEscape(t *testing.T) {
	rootDirectory := t.TempDir()
	outsideDirectory := t.TempDir()
	outsideFile := filepath.Join(outsideDirectory, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(rootDirectory, "secret.txt")); err != nil {
		t.Fatal(err)
	}

	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })

	response := httptest.NewRecorder()
	http.FileServerFS(root.FS()).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/secret.txt", nil))
	if response.Code == http.StatusOK {
		t.Fatalf("symlink escape returned status %d and body %q", response.Code, response.Body.String())
	}
}
