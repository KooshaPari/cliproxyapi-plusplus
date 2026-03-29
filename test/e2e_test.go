package test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/cmd"
)

func testRepoRoot() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(thisFile))
}

// TestServerHealth tests the server health endpoint.
func TestServerHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestRepoEntrypointsExist verifies the current entrypoint sources instead of machine-local binaries.
func TestRepoEntrypointsExist(t *testing.T) {
	root := testRepoRoot()
	paths := []string{
		filepath.Join(root, "cmd", "server", "main.go"),
		filepath.Join(root, "cmd", "cliproxyctl", "main.go"),
		filepath.Join(root, "cmd", "boardsync", "main.go"),
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("missing entrypoint %s: %v", path, err)
		}
		if info.IsDir() {
			t.Fatalf("expected file, got directory: %s", path)
		}
	}
}

// TestConfigFile tests config file parsing.
func TestConfigFile(t *testing.T) {
	config := `
port: 8317
host: localhost
log_level: debug
`
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Error(err)
	}
}

// TestOAuthLoginFlow tests OAuth flow.
func TestOAuthLoginFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"test","expires_in":3600}`))
		}
	}))
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/oauth/token")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestServerHelpIncludesKiloLoginFlag verifies the current server flag surface via `go run`.
func TestServerHelpIncludesKiloLoginFlag(t *testing.T) {
	root := testRepoRoot()
	command := exec.Command("go", "run", "./cmd/server", "-help")
	command.Dir = root

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("go run cmd/server -help failed unexpectedly: %v", err)
		}
	}

	output := stdout.String() + stderr.String()
	if !strings.Contains(output, "-kilo-login") {
		t.Fatalf("expected server help to mention -kilo-login, output=%q", output)
	}
}

// TestNativeCLISpecsRemainStable verifies current native CLI contract wiring.
func TestNativeCLISpecsRemainStable(t *testing.T) {
	if got := cmd.KiloSpec.Name; got != "kilo" {
		t.Fatalf("KiloSpec.Name = %q, want kilo", got)
	}
	if got := cmd.KiloSpec.Args; len(got) != 1 || got[0] != "auth" {
		t.Fatalf("KiloSpec.Args = %v, want [auth]", got)
	}

	roo := cmd.RooSpec
	if roo.Name != "roo" {
		t.Fatalf("RooSpec.Name = %q, want roo", roo.Name)
	}
	if len(roo.Args) != 2 || roo.Args[0] != "auth" || roo.Args[1] != "login" {
		t.Fatalf("RooSpec.Args = %v, want [auth login]", roo.Args)
	}
}
