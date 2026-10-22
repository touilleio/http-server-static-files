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

func TestStaticFileHandlerFallback(t *testing.T) {
	rootDirectory := t.TempDir()
	for name, content := range map[string]string{
		"index.html": "home",
		"file.txt":   "local file",
		"a b.txt":    "escaped file",
	} {
		if err := os.WriteFile(filepath.Join(rootDirectory, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(rootDirectory, "directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDirectory, "directory", "index.html"), []byte("directory index"), 0o600); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(rootDirectory, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("file.txt", filepath.Join(rootDirectory, "link.txt")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })

	const destination = "http://www.website2.com"
	for _, tc := range []struct {
		name, base, method, path, byteRange string
		status                              int
		location, body                      string
	}{
		{name: "existing file", base: destination, method: "GET", path: "/file.txt?download=1", status: 200, body: "local file"},
		{name: "existing index", base: destination, method: "GET", path: "/index.html", status: 200, body: "home"},
		{name: "escaped file", base: destination, method: "GET", path: "/a%20b.txt", status: 200, body: "escaped file"},
		{name: "existing HEAD", base: destination, method: "HEAD", path: "/file.txt", status: 200},
		{name: "byte range", base: destination, method: "GET", path: "/file.txt", byteRange: "bytes=0-4", status: 206, body: "local"},
		{name: "internal symlink", base: destination, method: "GET", path: "/link.txt", status: 200, body: "local file"},
		{name: "missing file", base: destination, method: "GET", path: "/old/page?lang=fr", status: 301, location: destination + "/old/page?lang=fr"},
		{name: "missing HEAD", base: destination, method: "HEAD", path: "/missing", status: 301, location: destination + "/missing"},
		{name: "root", base: destination, method: "GET", path: "/", status: 301, location: destination + "/"},
		{name: "directory", base: destination, method: "GET", path: "/directory", status: 301, location: destination + "/directory"},
		{name: "directory slash", base: destination, method: "GET", path: "/directory/", status: 301, location: destination + "/directory/"},
		{name: "directory index", base: destination, method: "GET", path: "/directory/index.html", status: 200, body: "directory index"},
		{name: "escaped redirect", base: destination, method: "GET", path: "/a%20b%3Fc%23d%2Fe?x=1%26y%3D2", status: 301, location: destination + "/a%20b%3Fc%23d%2Fe?x=1%26y%3D2"},
		{name: "empty query", base: destination, method: "GET", path: "/missing?", status: 301, location: destination + "/missing?"},
		{name: "fixed host", base: destination, method: "GET", path: "//other.example/missing", status: 301, location: destination + "//other.example/missing"},
		{name: "base path", base: destination + "/archive", method: "GET", path: "/old/page", status: 301, location: destination + "/archive/old/page"},
		{name: "symlink escape", base: destination, method: "GET", path: "/escape.txt", status: 301, location: destination + "/escape.txt"},
		{name: "traversal", base: destination, method: "GET", path: "/../secret.txt", status: 301, location: destination + "/../secret.txt"},
		{name: "POST", base: destination, method: "POST", path: "/missing", status: 405},
		{name: "disabled index", method: "GET", path: "/", status: 200, body: "home"},
		{name: "disabled missing", method: "GET", path: "/missing", status: 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := securityHeaders(envConfig{}, allowedMethods(staticFileHandler(root, tc.base)))
			request := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.byteRange != "" {
				request.Header.Set("Range", tc.byteRange)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d", response.Code, tc.status)
			}
			if got := response.Header().Get("Location"); got != tc.location {
				t.Errorf("Location = %q, want %q", got, tc.location)
			}
			if tc.status == 200 || tc.status == 206 || tc.method == "HEAD" {
				if got := response.Body.String(); got != tc.body {
					t.Errorf("body = %q, want %q", got, tc.body)
				}
			}
			if response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Error("security header missing")
			}
		})
	}
}

func TestFallbackRedirectURLConfig(t *testing.T) {
	for _, tc := range []struct {
		value, want string
		invalid     bool
	}{
		{value: "", want: ""},
		{value: "http://www.website2.com/", want: "http://www.website2.com"},
		{value: "https://example.com/archive/", want: "https://example.com/archive"},
		{value: "/relative", invalid: true},
		{value: "//example.com", invalid: true},
		{value: "ftp://example.com", invalid: true},
		{value: "http:///missing-host", invalid: true},
		{value: "https://user:password@example.com", invalid: true},
		{value: "https://example.com?query=1", invalid: true},
		{value: "https://example.com?", invalid: true},
		{value: "https://example.com#fragment", invalid: true},
		{value: "https://example.com#", invalid: true},
		{value: "https://example.com/\r\nInjected: header", invalid: true},
		{value: "https://example.com/%zz", invalid: true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("FALLBACK_REDIRECT_URL", tc.value)
			config, err := loadEnvConfig()
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid redirect URL accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if config.FallbackRedirectURL != tc.want {
				t.Fatalf("redirect URL = %q, want %q", config.FallbackRedirectURL, tc.want)
			}
		})
	}
}
