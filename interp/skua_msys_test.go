package interp

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMsysPath(t *testing.T) {
	if got := msysPath("/dev/null"); got != os.DevNull {
		t.Errorf("/dev/null: got %q", got)
	}
	if got := msysPath("out.txt"); got != "out.txt" {
		t.Errorf("out.txt: got %q", got)
	}
	if runtime.GOOS != "windows" {
		if got := msysPath("/tmp/x"); got != "/tmp/x" {
			t.Errorf("/tmp/x off Windows: got %q", got)
		}
		return
	}
	if got, want := msysPath("/tmp/sh9/x.txt"), filepath.Join(os.TempDir(), "sh9", "x.txt"); got != want {
		t.Errorf("/tmp/sh9/x.txt: got %q, want %q", got, want)
	}
	if got := msysPath("/tmpx"); got != "/tmpx" {
		t.Errorf("/tmpx: got %q", got)
	}
	sys := os.Getenv("SystemDrive") // "C:"
	letter := strings.ToLower(sys[:1])
	if got, want := msysPath("/"+letter+"/a/b.txt"), strings.ToUpper(letter)+`:\a\b.txt`; got != want {
		t.Errorf("drive path: got %q, want %q", got, want)
	}
}
