//go:build !windows

package winget

// RefreshPath only does something on Windows.
func RefreshPath() {}
