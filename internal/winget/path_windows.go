//go:build windows

package winget

import (
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const machineEnv = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`

// RefreshPath reloads PATH from the user and machine environment in the
// registry, so programs winget just installed are found by this process.
func RefreshPath() {
	var parts []string
	for _, k := range []struct {
		root registry.Key
		path string
	}{{registry.LOCAL_MACHINE, machineEnv}, {registry.CURRENT_USER, "Environment"}} {
		if v := readPath(k.root, k.path); v != "" {
			if exp, err := registry.ExpandString(v); err == nil {
				v = exp
			}
			parts = append(parts, strings.TrimRight(v, ";"))
		}
	}
	if len(parts) > 0 {
		os.Setenv("PATH", strings.Join(parts, ";"))
	}
}

func readPath(root registry.Key, path string) string {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, _ := k.GetStringValue("Path")
	return v
}

// AddUserPath appends dir to the user's PATH (HKCU\Environment) if missing.
func AddUserPath(dir string) (bool, error) {
	return editUserPath(func(list string) (string, bool) { return AppendPath(list, dir) })
}

// RemoveUserPath removes dir from the user's PATH.
func RemoveUserPath(dir string) (bool, error) {
	return editUserPath(func(list string) (string, bool) { return RemovePath(list, dir) })
}

func editUserPath(edit func(string) (string, bool)) (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	cur, _, err := k.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return false, err
	}
	next, changed := edit(cur)
	if !changed {
		return false, nil
	}
	if err := k.SetExpandStringValue("Path", next); err != nil {
		return false, err
	}
	BroadcastEnvChange()
	RefreshPath()
	return true, nil
}

var sendMessageTimeout = windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")

// BroadcastEnvChange tells Explorer and new terminals that the environment changed.
func BroadcastEnvChange() {
	const hwndBroadcast, wmSettingChange, smtoAbortIfHung = 0xffff, 0x001A, 0x0002
	env, _ := windows.UTF16PtrFromString("Environment")
	var result uintptr
	sendMessageTimeout.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(env)),
		smtoAbortIfHung, 5000, uintptr(unsafe.Pointer(&result)))
}
