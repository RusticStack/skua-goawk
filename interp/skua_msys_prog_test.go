package interp_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// An awk program writes to "/tmp/..." and reads it back with getline, as an agent does.
func TestMsysPathInProgram(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows only")
	}
	name := "skua-msys-test.txt"
	defer os.Remove(filepath.Join(os.TempDir(), name))
	src := `BEGIN { print "hello" > "/tmp/` + name + `"; close("/tmp/` + name + `");
		while ((getline line < "/tmp/` + name + `") > 0) print "got " line }`
	testGoAWK(t, src, "", "got hello\n", "", nil, nil)
}
