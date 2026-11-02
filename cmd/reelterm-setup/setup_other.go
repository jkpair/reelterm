//go:build !windows

package main

import "errors"

var errWindowsOnly = errors.New("reelterm-setup only runs on Windows")

func install(options) error   { return errWindowsOnly }
func uninstall(options) error { return errWindowsOnly }
