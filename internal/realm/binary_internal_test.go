package realm

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// stubLookups replaces the PATH, real-home and HOME lookups for one test.
func stubLookups(t *testing.T, pathHit, realHome, envHome string) {
	t.Helper()
	oldLook, oldReal, oldEnv := lookPath, realHomeDir, envHomeDir
	t.Cleanup(func() { lookPath, realHomeDir, envHomeDir = oldLook, oldReal, oldEnv })
	lookPath = func(string) (string, error) {
		if pathHit != "" {
			return pathHit, nil
		}
		return "", errors.New("not found")
	}
	realHomeDir = func() string { return realHome }
	envHomeDir = func() (string, error) { return envHome, nil }
}

func writeBin(t *testing.T, path string, mode os.FileMode) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFindBinaryPathHitWins(t *testing.T) {
	real := t.TempDir()
	writeBin(t, filepath.Join(real, ".local", "bin", "agy"), 0o755)
	stubLookups(t, "/on/path/agy", real, t.TempDir())
	if got := findBinary("agy", ".local/bin/agy"); got != "/on/path/agy" {
		t.Fatalf("got %q", got)
	}
}

func TestFindBinaryRealHomeWhileHOMERedirected(t *testing.T) {
	real := t.TempDir()
	want := writeBin(t, filepath.Join(real, ".local", "bin", "agy"), 0o755)
	stubLookups(t, "", real, t.TempDir())
	if got := findBinary("agy", ".local/bin/agy"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindBinaryFallsBackToHOME(t *testing.T) {
	env := t.TempDir()
	want := writeBin(t, filepath.Join(env, ".local", "bin", "claude"), 0o755)
	stubLookups(t, "", t.TempDir(), env)
	if got := findBinary("claude", ".local/bin/claude"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindBinarySkipsNonExecutable(t *testing.T) {
	real := t.TempDir()
	writeBin(t, filepath.Join(real, ".local", "bin", "agy"), 0o644)
	stubLookups(t, "", real, real)
	if got := findBinary("agy", ".local/bin/agy"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestFindBinaryNothingFound(t *testing.T) {
	stubLookups(t, "", t.TempDir(), t.TempDir())
	if got := findBinary("opencode", ".opencode/bin/opencode", ".local/bin/opencode"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestFindBinaryFallbackOrder(t *testing.T) {
	real, env := t.TempDir(), t.TempDir()
	writeBin(t, filepath.Join(real, ".local", "bin", "opencode"), 0o755)
	first := writeBin(t, filepath.Join(real, ".opencode", "bin", "opencode"), 0o755)
	writeBin(t, filepath.Join(env, ".opencode", "bin", "opencode"), 0o755)
	stubLookups(t, "", real, env)
	if got := findBinary("opencode", ".opencode/bin/opencode", ".local/bin/opencode"); got != first {
		t.Fatalf("got %q, want %q", got, first)
	}
	// Real home's later fallback still beats HOME's first one.
	if err := os.Remove(first); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(real, ".local", "bin", "opencode")
	if got := findBinary("opencode", ".opencode/bin/opencode", ".local/bin/opencode"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
