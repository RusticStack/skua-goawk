// Skua (RusticStack fork): file names inside an awk program as Git Bash's gawk (an MSYS
// program) resolves them on Windows.

package interp

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// msysPath maps a file name used inside an awk program (`print > "/tmp/x"`,
// `getline < "/tmp/x"`, `close("/tmp/x")`) the way MSYS gawk in Git Bash resolves it, and the
// way Skua Bash maps POSIX path arguments: "/dev/null" is NUL, "/tmp/x" is under the temp dir,
// "/c/x" is C:\x when that drive exists. Other names, and every name off Windows, are
// returned unchanged. "/dev/stdout", "/dev/stderr" and "/dev/stdin" are handled by the
// callers (they are the standard streams).
func msysPath(name string) string {
	if name == "/dev/null" {
		return os.DevNull
	}
	if runtime.GOOS != "windows" {
		return name
	}
	if rest, ok := strings.CutPrefix(name, "/tmp"); ok && (rest == "" || rest[0] == '/') {
		rest = strings.TrimLeft(rest, "/")
		dir := os.TempDir()
		if rest == "" {
			return dir
		}
		return filepath.Join(dir, filepath.FromSlash(rest))
	}
	if len(name) >= 3 && name[0] == '/' && name[2] == '/' && isASCIILetter(name[1]) {
		drive := strings.ToUpper(name[1:2]) + `:\`
		if _, err := os.Stat(drive); err == nil {
			return drive + filepath.FromSlash(name[3:])
		}
	}
	return name
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
