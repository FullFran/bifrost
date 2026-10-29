package realm_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/FullFran/bifrost/internal/realm"
)

func TestClaudeCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", home)
	bin := filepath.Join(home, ".local", "bin", "claude")
	if err := os.MkdirAll(filepath.Dir(bin), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 99\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r, ok := realm.Get("claude")
	if !ok {
		t.Fatal("claude not registered")
	}
	if available, _ := r.IsAvailable(context.Background()); available {
		t.Fatal("non-executable fallback reported available")
	}
	if err := os.Chmod(bin, 0755); err != nil {
		t.Fatal(err)
	}
	if available, path := r.IsAvailable(context.Background()); !available || path != bin {
		t.Fatalf("availability: %t %s", available, path)
	}
	for _, tt := range []struct {
		name string
		task realm.Task
		args []string
	}{
		{
			"native defaults",
			realm.Task{Prompt: "two words; not a shell command"},
			[]string{"--dangerously-skip-permissions", "-p", "two words; not a shell command"},
		},
		{
			"model effort",
			realm.Task{Prompt: "hello", Model: "sonnet", Effort: "high"},
			[]string{"--dangerously-skip-permissions", "-p", "hello", "--model", "sonnet", "--effort", "high"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := r.BuildCommand(context.Background(), tt.task, home)
			if err != nil {
				t.Fatal(err)
			}
			if cmd.Path != bin || cmd.Dir != home || !reflect.DeepEqual(cmd.Args[1:], tt.args) {
				t.Fatalf("command: %+v", cmd)
			}
			// Intentionally do not run the command: invocation is covered with CLI fakes.
		})
	}
}
