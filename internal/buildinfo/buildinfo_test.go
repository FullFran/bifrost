package buildinfo_test

import (
	"strings"
	"testing"

	"github.com/FullFran/bifrost/internal/buildinfo"
)

func TestBuildInfo(t *testing.T) {
	v := buildinfo.Version()
	if v == "" {
		t.Errorf("expected non-empty version")
	}
	if strings.HasPrefix(v, "v") {
		t.Errorf("expected version to be bare without leading 'v', got '%s'", v)
	}

	full := buildinfo.Full()
	if !strings.Contains(full, "Bifrost") {
		t.Errorf("expected full banner to contain 'Bifrost', got '%s'", full)
	}
}
