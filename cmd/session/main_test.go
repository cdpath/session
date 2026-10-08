package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	for _, flag := range []string{"-v", "--version"} {
		var out, errOut bytes.Buffer
		if code := run([]string{flag}, &out, &errOut); code != 0 {
			t.Fatalf("%s: exit %d, stderr %q", flag, code, errOut.String())
		}
		if !strings.HasPrefix(out.String(), "session ") || strings.Count(out.String(), "\n") != 1 {
			t.Errorf("%s: output %q, want one line starting with \"session \"", flag, out.String())
		}
	}
}
