package realm

import (
	"reflect"
	"testing"
)

func TestAgyArgsModelAndEffort(t *testing.T) {
	cases := []struct {
		name   string
		model  string
		effort string
		want   []string
	}{
		{"agent defaults", "", "", nil},
		{"model only", "gemini-3.8-flash-high", "", []string{"--model", "gemini-3.8-flash-high"}},
		{"effort only", "", "high", []string{"--effort", "high"}},
		{"effort rewrites model suffix", "gemini-3.8-flash-high", "low", []string{"--model", "gemini-3.8-flash-low"}},
		{"matching suffix", "gemini-3.8-flash-medium", "medium", []string{"--model", "gemini-3.8-flash-medium"}},
		{"model without suffix", "custom-model", "high", []string{"--model", "custom-model", "--effort", "high"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := agyModelArgs(tc.model, tc.effort)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("agyModelArgs(%q, %q) = %v, want %v", tc.model, tc.effort, got, tc.want)
			}
		})
	}
}

func TestAgyArgsStreamJSON(t *testing.T) {
	got := agyArgs(Task{Prompt: "do it", StreamJSON: true})
	want := []string{"--dangerously-skip-permissions", "-p", "do it", "--output-format", "stream-json"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("agyArgs = %v, want %v", got, want)
	}
	plain := agyArgs(Task{Prompt: "do it", Model: "gemini-3.8-flash-high", Effort: "low"})
	wantPlain := []string{"--dangerously-skip-permissions", "-p", "do it", "--model", "gemini-3.8-flash-low"}
	if !reflect.DeepEqual(plain, wantPlain) {
		t.Errorf("agyArgs = %v, want %v", plain, wantPlain)
	}
}

func TestOpenCodeArgsStreamJSON(t *testing.T) {
	got := openCodeArgs(Task{Prompt: "do it", StreamJSON: true})
	want := []string{"run", "--auto", "--format", "json", "do it"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("openCodeArgs = %v, want %v", got, want)
	}
}
