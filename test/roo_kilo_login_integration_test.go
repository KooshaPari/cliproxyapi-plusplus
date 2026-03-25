package test

import (
	"bytes"
	"strings"
	"testing"

	cliproxycmd "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/cmd"
)

func TestRooLoginRunner_WithFakeRoo(t *testing.T) {
	mockRunner := func(spec cliproxycmd.NativeCLISpec) (int, error) {
		if spec.Name != "roo" {
			t.Fatalf("spec.Name = %q, want roo", spec.Name)
		}
		return 0, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cliproxycmd.RunRooLoginWithRunner(mockRunner, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunRooLoginWithRunner() = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Roo authentication successful") {
		t.Fatalf("stdout missing success message: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestKiloLoginRunner_WithFakeKilo(t *testing.T) {
	mockRunner := func(spec cliproxycmd.NativeCLISpec) (int, error) {
		if spec.Name != "kilo" {
			t.Fatalf("spec.Name = %q, want kilo", spec.Name)
		}
		return 0, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cliproxycmd.RunKiloLoginWithRunner(mockRunner, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunKiloLoginWithRunner() = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Kilo authentication successful") {
		t.Fatalf("stdout missing success message: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestThegentLoginRunner_WithoutCLI_ExitsNonZero(t *testing.T) {
	mockRunner := func(spec cliproxycmd.NativeCLISpec) (int, error) {
		if spec.Name != "thegent" {
			t.Fatalf("spec.Name = %q, want thegent", spec.Name)
		}
		return -1, assertErr("thegent CLI not found")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cliproxycmd.RunThegentLoginWithRunner(mockRunner, &stdout, &stderr, "codex")
	if code != 1 {
		t.Fatalf("RunThegentLoginWithRunner() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Install:") {
		t.Fatalf("stderr missing install hint: %q", stderr.String())
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
