// Package buildinfo carries the identity of the running Bifrost binary: which version
// it is, which commit it was built from, and when.
package buildinfo

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// defaultVersion is the version this source tree claims to be. Bump it in the
// release commit; .github/workflows/release.yml asserts it against the tag.
const defaultVersion = "0.2.0"

const unknownValue = "unknown"

// Injected at link time by GoReleaser via ldflags.
var (
	version string
	commit  string
	date    string
)

// Version reports the semantic version without a leading "v".
func Version() string {
	return resolveVersion(version, moduleVersion())
}

// Commit reports the short commit the binary was built from, or "unknown".
func Commit() string {
	return resolveCommit(commit, vcsSetting("vcs.revision"))
}

// Date reports the build timestamp, or "unknown".
func Date() string {
	return resolveDate(date, vcsSetting("vcs.time"))
}

// Full returns a formatted multiline version banner.
func Full() string {
	v := Version()
	c := Commit()
	d := Date()
	return fmt.Sprintf("Bifrost v%s (commit: %s, date: %s)", v, c, d)
}

func resolveVersion(injected, modVer string) string {
	if injected != "" {
		return strings.TrimPrefix(injected, "v")
	}
	if modVer != "" && modVer != "(devel)" {
		return strings.TrimPrefix(modVer, "v")
	}
	return defaultVersion
}

func resolveCommit(injected, vcsRev string) string {
	if injected != "" {
		return injected
	}
	if vcsRev != "" {
		if len(vcsRev) > 8 {
			return vcsRev[:8]
		}
		return vcsRev
	}
	return unknownValue
}

func resolveDate(injected, vcsTime string) string {
	if injected != "" {
		return injected
	}
	if vcsTime != "" {
		return vcsTime
	}
	return unknownValue
}

func moduleVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok || bi.Main.Version == "" {
		return ""
	}
	return bi.Main.Version
}

func vcsSetting(key string) string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range bi.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}
