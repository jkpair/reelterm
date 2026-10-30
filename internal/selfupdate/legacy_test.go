package selfupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLinkBeside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no symlinks on Windows")
	}
	d := t.TempDir()
	exe := filepath.Join(d, "yt-x")
	os.WriteFile(exe, []byte("bin"), 0o755)
	link, ok := linkBeside(exe, "yt-x", "reelterm")
	if !ok || link != filepath.Join(d, "reelterm") {
		t.Fatalf("link = %q, ok = %v", link, ok)
	}
	if target, err := os.Readlink(link); err != nil || target != "yt-x" {
		t.Fatalf("readlink = %q, %v", target, err)
	}
	if again, ok := linkBeside(exe, "yt-x", "reelterm"); !ok || again != link {
		t.Fatalf("second call = %q, %v", again, ok)
	}
	if _, ok := linkBeside(filepath.Join(d, "reelterm"), "yt-x", "reelterm"); ok {
		t.Fatal("reelterm reported as legacy")
	}
}
