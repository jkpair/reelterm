package winget

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestInstallArgs(t *testing.T) {
	got := InstallArgs(Mpv, true)
	want := []string{"install", "-e", "--id", "shinchiro.mpv", "--scope", "user", "--silent", "--accept-package-agreements", "--accept-source-agreements"}
	if !slices.Equal(got, want) {
		t.Fatalf("args = %q", got)
	}
	if slices.Contains(InstallArgs(Mpv, false), "--scope") {
		t.Fatal("machine scope still passes --scope")
	}
}

func TestAppendRemovePath(t *testing.T) {
	dir := `C:\Users\a\AppData\Local\Programs\reelterm`
	list, changed := AppendPath(`C:\Windows;C:\bin;`, dir)
	if !changed || list != `C:\Windows;C:\bin;`+dir {
		t.Fatalf("append = %q", list)
	}
	if _, changed := AppendPath(list, `c:\users\A\appdata\local\programs\reelterm\`); changed {
		t.Fatal("appended a duplicate")
	}
	if l, _ := AppendPath("", dir); l != dir {
		t.Fatalf("append to empty = %q", l)
	}
	list, changed = RemovePath(list, dir)
	if !changed || list != `C:\Windows;C:\bin` {
		t.Fatalf("remove = %q", list)
	}
}

func TestFindInPackage(t *testing.T) {
	d := t.TempDir()
	exe := filepath.Join(d, "Packages", "shinchiro.mpv_Microsoft.Winget.Source_8wekyb3d8bbwe", "mpv", "mpv.exe")
	os.MkdirAll(filepath.Dir(exe), 0o755)
	os.WriteFile(exe, nil, 0o755)
	if got := FindInPackage(d, Mpv, "MPV.exe"); got != exe {
		t.Fatalf("found %q", got)
	}
	if got := FindInPackage(d, VLC, "vlc.exe"); got != "" {
		t.Fatalf("found %q", got)
	}
}
