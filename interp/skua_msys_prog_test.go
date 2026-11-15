package interp_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/benhoyt/goawk/interp"
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

func TestSkuaIgnoreCase(t *testing.T) {
	src := `BEGIN { IGNORECASE = 1 }
/hello/ {
	s = $0
	sub(/WORLD/, "earth", s)
	n = split("aXbxc", parts, "x")
	print "matched", ($0 ~ /HELLO/), index($0, "world"), s, n, parts[2]
	print index("ȺȺȺa", "a"), index("İİİa", "a")
	IGNORECASE = 0
	print ($0 ~ /hello/)
}`
	testGoAWK(t, src, "Hello World\n", "matched 1 7 Hello earth 3 b\n7 7\n0\n", "", nil, nil)
	testGoAWK(t, src, "Hello World\n", "matched 1 7 Hello earth 3 b\n4 4\n0\n", "", nil, func(c *interp.Config) { c.Chars = true })
}
