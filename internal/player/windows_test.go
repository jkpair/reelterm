//go:build windows

package player

import (
	"os/exec"
	"strings"
	"testing"
)

func TestWindowsShellCommand(t *testing.T) {
	out, err := ShellCommand("echo hello & echo world").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Fields(string(out)); len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Fatalf("output = %q", out)
	}
}

func TestWindowsEditorFallback(t *testing.T) {
	cmd, err := EditorCmd("no-such-editor", `C:\x.conf`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(strings.TrimSuffix(strings.ToLower(cmd.Path[strings.LastIndexAny(cmd.Path, `\/`)+1:]), ".exe"), "notepad") {
		t.Fatalf("editor = %s", cmd.Path)
	}
}

func TestWindowsCopy(t *testing.T) {
	if _, err := exec.LookPath("clip"); err != nil {
		t.Skip("clip.exe not available")
	}
	if err := Copy("héllo 🌧️ https://youtu.be/x?a=1&b=2"); err != nil {
		t.Fatal(err)
	}
}
