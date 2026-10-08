package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// version can be set at build time with -ldflags "-X main.version=v1.2.3".
// Otherwise it comes from the module version recorded by go install.
var version = ""

func versionString() string {
	v := version
	var rev, at string
	var dirty bool
	if info, ok := debug.ReadBuildInfo(); ok {
		if v == "" {
			v = info.Main.Version
		}
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.time":
				at = s.Value
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
	}
	if v == "" {
		v = "(devel)"
	}
	out := "session " + v
	if len(rev) > 12 {
		rev = rev[:12]
	}
	// Pseudo-versions already embed the commit; older toolchains report
	// "(devel)" for local builds, where the commit is the only identifier.
	if rev != "" && !strings.Contains(v, rev) {
		out += " " + rev
		if dirty {
			out += "-dirty"
		}
		if at != "" {
			out += " " + at
		}
	}
	return fmt.Sprintf("%s (%s %s/%s)", out, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
