package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestCLIHelper(t *testing.T) {
	if os.Getenv("BIFROST_TEST_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"bifrost"}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
	os.Exit(2)
}

type cliFixture struct {
	t         *testing.T
	home, bin string
}

func newCLI(t *testing.T) *cliFixture {
	t.Helper()
	root := t.TempDir()
	f := &cliFixture{t, root, filepath.Join(root, "bin")}
	if err := os.MkdirAll(f.bin, 0755); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"agy", "opencode", "claude"} {
		// This fake prints argv only; it never invokes an agent or reads credentials.
		script := "#!/bin/sh\nprintf '%s\\n' '" + id + "' \"$@\"\n"
		if err := os.WriteFile(filepath.Join(f.bin, id), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f *cliFixture) run(args ...string) (string, string, int) {
	f.t.Helper()
	exe, err := os.Executable()
	if err != nil {
		f.t.Fatal(err)
	}
	cmd := exec.Command(exe, append([]string{"-test.run=^TestCLIHelper$", "--"}, args...)...)
	cmd.Env = []string{
		"BIFROST_TEST_HELPER=1", "HOME=" + f.home, "PATH=" + f.bin,
		"XDG_CACHE_HOME=" + filepath.Join(f.home, "cache"),
	}
	cmd.Dir = f.home
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else {
			f.t.Fatal(err)
		}
	}
	return out.String(), stderr.String(), code
}

func (f *cliFixture) ok(want string, args ...string) {
	f.t.Helper()
	out, stderr, code := f.run(args...)
	if code != 0 || stderr != "" || out != want {
		f.t.Fatalf("%v: code=%d stdout=%q stderr=%q; want %q", args, code, out, stderr, want)
	}
}

func (f *cliFixture) snapshot() string {
	f.t.Helper()
	out, stderr, code := f.run("config", "--json")
	if code != 0 || stderr != "" || !json.Valid([]byte(out)) {
		f.t.Fatalf("config --json: code=%d stdout=%q stderr=%q", code, out, stderr)
	}
	return out
}

func (f *cliFixture) reject(want string, args ...string) {
	f.t.Helper()
	before := f.snapshot()
	f.ok("No Bifrost jobs found.\n", "list")
	out, stderr, code := f.run(args...)
	if code != 1 || out != "" || stderr != want {
		f.t.Fatalf("%v: code=%d stdout=%q stderr=%q; want %q", args, code, out, stderr, want)
	}
	if after := f.snapshot(); after != before {
		f.t.Fatal("rejection changed configuration")
	}
	// Check job counters through the CLI, not internal storage.
	f.ok("No Bifrost jobs found.\n", "list")
}

func (f *cliFixture) dispatch(want string, args ...string) {
	f.t.Helper()
	before := f.snapshot()
	out, stderr, code := f.run(args...)
	// Only task IDs, realm names, and elapsed time vary in successful diagnostics.
	diagnostics := `^⚡ Bifrost crossing to \[[^\n]+\] with task bf_[0-9]{8}_[0-9]{6}_[0-9a-f]{8}\n\n✨ Task completed successfully in [0-9.a-zµ]+\.\n$`
	if code != 0 || out != want || !regexp.MustCompile(diagnostics).MatchString(stderr) {
		f.t.Fatalf("dispatch %v: code=%d stdout=%q stderr=%q; want %q", args, code, out, stderr, want)
	}
	if f.snapshot() != before {
		f.t.Fatal("dispatch changed config")
	}
}

func TestCLIConfigJSON(t *testing.T) {
	f := newCLI(t)
	var cfg struct {
		Priority []string `json:"priority"`
		Realms   map[string]struct {
			Enabled bool `json:"enabled"`
			Auto    bool `json:"auto"`
		} `json:"realms"`
	}
	if err := json.Unmarshal([]byte(f.snapshot()), &cfg); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Priority, []string{"agy", "opencode", "claude"}) {
		t.Fatalf("priority: %v", cfg.Priority)
	}
	for _, id := range cfg.Priority {
		if !cfg.Realms[id].Enabled || !cfg.Realms[id].Auto {
			t.Fatalf("missing effective defaults for %s", id)
		}
	}
	out, stderr, code := f.run("config")
	if code != 0 || stderr != "" || !strings.HasPrefix(out, "🌈 Bifrost Declarative Config (") {
		t.Fatalf("legacy config display: %d %q %q", code, out, stderr)
	}
	// A pre-controls file keeps its priority and opt-out defaults.
	path := filepath.Join(f.home, ".config", "bifrost", "config.json")
	data := `{"priority":["opencode","agy"],"realms":{"opencode":{"default_model":"local"}}}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	f.dispatch("opencode\nrun\n--auto\n--format\njson\n--model\nlocal\nhello\n", "--json", "hello")
}

func TestCLIRealmControls(t *testing.T) {
	for _, id := range []string{"agy", "opencode", "claude"} {
		t.Run(id, func(t *testing.T) {
			f := newCLI(t)
			f.snapshot()
			f.ok("✅ "+id+" enabled = false\n", "config", "set", id, "enabled", "false")
			f.reject("Bridge routing error: realm '"+id+"' is disabled\n", "--json", "-r", id, "hello")
			f.reject("Bridge routing error: realm '"+id+"' is disabled\n", "--json", "-w", "-b", "-r", id, "hello")
			f.ok("✅ "+id+" enabled = true\n", "config", "set", id, "enabled", "true")
			f.ok("✅ "+id+" auto = false\n", "config", "set", id, "auto", "false")
			f.reject("Error: invalid value \"yes\" for enabled (expected true or false)\n", "config", "set", id, "enabled", "yes")
			f.reject("Error: invalid value \"\" for auto (expected true or false)\n", "config", "set", id, "auto", "")
			f.reject("Error: invalid value \"TRUE\" for auto (expected true or false)\n", "config", "set", id, "auto", "TRUE")
			f.reject("Error: unknown realm \"unknown\"\n", "config", "set", "unknown", "auto", "false")
			for _, other := range []string{"agy", "opencode", "claude"} {
				if other != id {
					f.ok("✅ "+other+" enabled = false\n", "config", "set", other, "enabled", "false")
				}
			}
			f.reject("Bridge routing error: no operational realms found eligible for auto. Install 'agy', 'opencode', or 'claude' and enable automatic routing\n", "--json", "hello")
			want := map[string]string{
				"agy":      "agy\n--dangerously-skip-permissions\n-p\nhello\n--output-format\nstream-json\n",
				"opencode": "opencode\nrun\n--auto\n--format\njson\nhello\n",
				"claude":   claudeStream,
			}[id]
			f.dispatch(want, "--json", "-r", id, "hello")
			f.ok("✅ "+id+" auto unset (using the agent's own default)\n", "config", "unset", id, "auto")
			f.dispatch(want, "--json", "hello")
			f.ok("✅ "+id+" enabled = false\n", "config", "set", id, "enabled", "false")
			f.ok("✅ "+id+" enabled unset (using the agent's own default)\n", "config", "unset", id, "enabled")
			f.dispatch(want, "--json", "-r", id, "hello")
		})
	}
}

const claudeStream = "claude\n--dangerously-skip-permissions\n-p\nhello\n--output-format\nstream-json\n--verbose\n--include-partial-messages\n"

func TestCLIClaudeDispatch(t *testing.T) {
	f := newCLI(t)
	f.dispatch(claudeStream, "--json", "-r", "claude", "hello")
	f.ok("✅ claude model = sonnet\n", "config", "set", "claude", "model", "sonnet")
	f.ok("✅ claude effort = high\n", "config", "set", "claude", "effort", "high")
	f.ok("✅ claude auto = false\n", "config", "set", "claude", "auto", "false")
	f.dispatch(claudeStream+"--model\nsonnet\n--effort\nhigh\n", "--json", "-r", "claude", "hello")
	f.dispatch(claudeStream+"--model\nopus\n--effort\nlow\n", "--json", "-r", "claude", "-m", "opus", "-e", "low", "hello")
	f.ok("✅ claude model unset (using the agent's own default)\n", "config", "unset", "claude", "model")
}

func TestCLIAutoPolicy(t *testing.T) {
	f := newCLI(t)
	f.ok("✅ agy enabled = false\n", "config", "set", "agy", "enabled", "false")
	f.ok("✅ opencode auto = false\n", "config", "set", "opencode", "auto", "false")
	f.dispatch(claudeStream, "--json", "hello")
	f.ok("✅ claude auto = false\n", "config", "set", "claude", "auto", "false")
	f.reject("Bridge routing error: no operational realms found eligible for auto. Install 'agy', 'opencode', or 'claude' and enable automatic routing\n", "--json", "hello")
}

func TestCLILegacyDefaultsAndDoctor(t *testing.T) {
	f := newCLI(t)
	f.dispatch("agy\n--dangerously-skip-permissions\n-p\nhello\n--output-format\nstream-json\n", "--json", "hello")
	f.ok("✅ agy model = gemini-high\n", "config", "set", "agy", "model", "gemini-high")
	f.ok("✅ agy effort = low\n", "config", "set", "agy", "effort", "low")
	f.dispatch("agy\n--dangerously-skip-permissions\n-p\nhello\n--output-format\nstream-json\n--model\ngemini-low\n", "--json", "-r", "agy", "hello")
	f.dispatch("opencode\nrun\n--auto\n--format\njson\n--model\nlocal\n--variant\nhigh\nhello\n", "--json", "-r", "opencode", "-m", "local", "-e", "high", "hello")
	f.ok("✅ claude enabled = false\n", "config", "set", "claude", "enabled", "false")
	out, stderr, code := f.run("doctor")
	if code != 0 || stderr != "" || !strings.Contains(out, "ENABLED") || !strings.Contains(out, "AUTO") {
		t.Fatalf("doctor: %d %q %q", code, out, stderr)
	}
	found := false
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "claude ") {
			found = true
			if !strings.Contains(line, "Operational") || !strings.Contains(line, "false") {
				t.Fatalf("availability/policy conflated: %s", line)
			}
		}
	}
	if !found {
		t.Fatal("doctor omitted Claude")
	}
}

func TestCLIRegistryFallbackPolicy(t *testing.T) {
	f := newCLI(t)
	f.snapshot()
	path := filepath.Join(f.home, ".config", "bifrost", "config.json")
	data := `{"priority":["unknown","agy","agy"],"realms":{"agy":{"enabled":false},"opencode":{"auto":false}}}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	f.dispatch(claudeStream, "--json", "hello")
	after, err := os.ReadFile(path)
	if err != nil || string(after) != data {
		t.Fatal("registry fallback rewrote configuration")
	}
}

func TestCLILegacyNullConfig(t *testing.T) {
	for _, data := range []string{
		`{"realms":{"agy":{"default_model":null}}}`,
		`{"realms":{"agy":{"default_effort":null}}}`,
		`{"priority":null}`,
		`{"worktree":null}`,
		`{"default_realm":null}`,
		`{"fallback_on_error":null}`,
	} {
		t.Run(data, func(t *testing.T) {
			f := newCLI(t)
			f.snapshot()
			path := filepath.Join(f.home, ".config", "bifrost", "config.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			out, stderr, code := f.run("--json", "hello")
			want := "agy\n--dangerously-skip-permissions\n-p\nhello\n--output-format\nstream-json\n"
			diagnostics := `^⚡ Bifrost crossing to \[Asgard \(Google Antigravity\)\] with task bf_[0-9]{8}_[0-9]{6}_[0-9a-f]{8}\n\n✨ Task completed successfully in [0-9.a-zµ]+\.\n$`
			if code != 0 || out != want || !regexp.MustCompile(diagnostics).MatchString(stderr) {
				t.Fatalf("dispatch: code=%d stdout=%q stderr=%q; want %q", code, out, stderr, want)
			}
			var cfg struct {
				DefaultRealm    string   `json:"default_realm"`
				Priority        []string `json:"priority"`
				FallbackOnError bool     `json:"fallback_on_error"`
				Worktree        struct {
					AutoCleanupOnMerge bool `json:"auto_cleanup_on_merge"`
				} `json:"worktree"`
			}
			if err := json.Unmarshal([]byte(f.snapshot()), &cfg); err != nil {
				t.Fatal(err)
			}
			if cfg.DefaultRealm != "auto" || !cfg.FallbackOnError || !cfg.Worktree.AutoCleanupOnMerge {
				t.Fatalf("legacy null changed scalar/struct defaults: %+v", cfg)
			}
			if data == `{"priority":null}` {
				if cfg.Priority != nil {
					t.Fatalf("null priority must stay nil: %v", cfg.Priority)
				}
			} else if !reflect.DeepEqual(cfg.Priority, []string{"agy", "opencode", "claude"}) {
				t.Fatalf("unexpected priority: %v", cfg.Priority)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != data {
				t.Fatal("dispatch rewrote legacy configuration")
			}
		})
	}
}

func TestCLIInvalidConfigFailsClosed(t *testing.T) {
	for _, data := range []string{
		`{`, `{"realms":{"claude":{"enabled":"false"}}}`,
		`{"realms":{"agy":{"auto":null}}}`, `{"realms":{"agy":{"enabled":null}}}`,
		`{"realms":{"agy":{"auto":"true"}}}`, `{"realms":null}`,
		`{"realms":{"claude":null}}`, `null`, " null \n",
	} {
		t.Run(data, func(t *testing.T) {
			f := newCLI(t)
			f.snapshot()
			path := filepath.Join(f.home, ".config", "bifrost", "config.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{
				{"--json", "hello"}, {"--json", "-r", "claude", "hello"},
				{"config", "set", "agy", "enabled", "true"},
			} {
				out, stderr, code := f.run(args...)
				if code != 1 || out != "" || !strings.HasPrefix(stderr, "Error: failed to load config: ") {
					t.Fatalf("%v: code=%d stdout=%q stderr=%q", args, code, out, stderr)
				}
				after, err := os.ReadFile(path)
				if err != nil || string(after) != data {
					t.Fatal("invalid config overwritten")
				}
			}
		})
	}
}
