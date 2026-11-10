package realm

import "testing"

// SetRealHomeForTest points the real-home lookup at dir for the duration of t,
// so tests never depend on what is installed in the machine's real home.
func SetRealHomeForTest(t *testing.T, dir string) {
	t.Helper()
	old := realHomeDir
	realHomeDir = func() string { return dir }
	t.Cleanup(func() { realHomeDir = old })
}
