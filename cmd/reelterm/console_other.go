//go:build !windows

package main

func relaunchInTerminal() bool { return false }

func pauseOnError() {}
