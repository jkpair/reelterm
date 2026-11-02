//go:build windows

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getConsoleProcessList = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleProcessList")

// ownConsole reports whether reelterm is the only process in its console,
// which is the case when it was started by double-clicking the .exe.
func ownConsole() bool {
	var ids [2]uint32
	n, _, _ := getConsoleProcessList.Call(uintptr(unsafe.Pointer(&ids[0])), uintptr(len(ids)))
	return n == 1
}

// relaunchInTerminal offers to reopen reelterm in Windows Terminal when it
// was double-clicked into the legacy console host, which draws the TUI and
// images worse. It returns true when reelterm was relaunched and this
// process should exit.
func relaunchInTerminal() bool {
	if os.Getenv("WT_SESSION") != "" || !ownConsole() {
		return false
	}
	wt, err := exec.LookPath("wt")
	if err != nil {
		fmt.Println("Tip: reelterm looks best in Windows Terminal (winget install Microsoft.WindowsTerminal).")
		return false
	}
	fmt.Print("reelterm looks best in Windows Terminal. Open it there? [Y/n]: ")
	ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if a := strings.ToLower(strings.TrimSpace(ans)); a != "" && a != "y" && a != "yes" {
		return false
	}
	var cmd *exec.Cmd
	frag := filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Windows Terminal", "Fragments", "reelterm", "reelterm.json")
	if _, err := os.Stat(frag); err == nil && len(os.Args) == 1 {
		// The profile reelterm-setup added, with the Nerd Font.
		cmd = exec.Command(wt, "-p", "reelterm")
	} else {
		exe, err := os.Executable()
		if err != nil {
			return false
		}
		cmd = exec.Command(wt, append([]string{exe}, os.Args[1:]...)...)
	}
	return cmd.Start() == nil
}

// pauseOnError keeps a double-clicked console open so an error can be read.
func pauseOnError() {
	if ownConsole() {
		fmt.Fprint(os.Stderr, "\nPress Enter to close.")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}
}
