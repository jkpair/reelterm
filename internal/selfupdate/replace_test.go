package selfupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceExe(t *testing.T) {
	d := t.TempDir()
	exe, next := filepath.Join(d, "reelterm.exe"), filepath.Join(d, "new")
	os.WriteFile(exe, []byte("old"), 0o755)
	os.WriteFile(next, []byte("new"), 0o755)
	if err := replaceExe(exe, next); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "new" {
		t.Fatalf("exe = %q", b)
	}
}

func TestOldExe(t *testing.T) {
	for in, want := range map[string]string{
		`C:\x\reelterm.exe`: `C:\x\reelterm.old.exe`,
		`C:\x\REELTERM.EXE`: `C:\x\REELTERM.old.exe`,
		"/usr/bin/reelterm": "/usr/bin/reelterm.old.exe",
	} {
		if got := oldExe(in); got != want {
			t.Errorf("oldExe(%q) = %q", in, got)
		}
	}
}
