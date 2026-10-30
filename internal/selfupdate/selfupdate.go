// Package selfupdate checks GitHub releases and replaces the running binary.
package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

var client = &http.Client{Timeout: 30 * time.Second}

func latest(ctx context.Context, repo string) (*release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: %s", resp.Status)
	}
	var r release
	return &r, json.NewDecoder(resp.Body).Decode(&r)
}

// LatestTag returns the latest release tag of repo (owner/name).
func LatestTag(ctx context.Context, repo string) (string, error) {
	r, err := latest(ctx, repo)
	if err != nil {
		return "", err
	}
	return r.TagName, nil
}

func parts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v, _, _ = strings.Cut(v, "-")
	var out []int
	for _, p := range strings.Split(v, ".") {
		n, _ := strconv.Atoi(p)
		out = append(out, n)
	}
	return out
}

// Newer reports whether version a is newer than b.
func Newer(a, b string) bool {
	pa, pb := parts(a), parts(b)
	for i := range max(len(pa), len(pb)) {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// AssetName is the release asset for this platform.
func AssetName(name string) string {
	n := fmt.Sprintf("%s-%s-%s", name, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		n += ".exe"
	}
	return n
}

// Update downloads the latest release asset and atomically replaces the
// running executable. It returns the new tag, or "" if already current.
func Update(ctx context.Context, repo, name, current string) (string, error) {
	r, err := latest(ctx, repo)
	if err != nil {
		return "", err
	}
	if !Newer(r.TagName, current) {
		return "", nil
	}
	want := AssetName(name)
	var url string
	for _, a := range r.Assets {
		if a.Name == want {
			url = a.URL
		}
	}
	if url == "" {
		return "", fmt.Errorf("release %s has no asset %s", r.TagName, want)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return "", err
	}
	if strings.HasPrefix(exe, "/nix/store/") {
		return "", errors.New("installed via Nix; update with your Nix profile instead")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: %s", resp.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(exe), ".yt-x-update-*")
	if err != nil {
		return "", fmt.Errorf("cannot write next to %s (try sudo): %w", exe, err)
	}
	defer os.Remove(tmp.Name())
	n, err := io.Copy(tmp, resp.Body)
	tmp.Close()
	if err != nil {
		return "", err
	}
	if n < 1<<20 {
		return "", errors.New("downloaded file is too small, not updating")
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), exe); err != nil {
		return "", err
	}
	return r.TagName, nil
}
