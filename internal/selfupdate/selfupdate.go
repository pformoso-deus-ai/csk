// Package selfupdate downloads + installs the latest csk release from
// GitHub.
//
// Wire format we depend on:
//   - https://api.github.com/repos/<owner>/<repo>/releases/latest
//     returns JSON with tag_name and an assets array. Each asset has a
//     name and a browser_download_url.
//   - Goreleaser produces assets named csk_<ver>_<os>_<arch>.{tar.gz|zip}
//     and a checksums.txt SHA256 file. We verify before installing.
package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DefaultOwner / DefaultRepo are baked into the binary so `csk upgrade`
// always knows where to look. Tests can override via Client.Owner/Repo.
const (
	DefaultOwner = "pformoso-deus-ai"
	DefaultRepo  = "csk"
)

// Release is the slice of GitHub's release JSON we care about.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Client fetches and installs releases.
type Client struct {
	HTTP    *http.Client
	APIBase string // default: https://api.github.com
	Owner   string
	Repo    string
}

// New returns a Client wired for the real GitHub API.
func New() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 60 * time.Second},
		APIBase: "https://api.github.com",
		Owner:   DefaultOwner,
		Repo:    DefaultRepo,
	}
}

// LatestRelease returns the most recent published release.
func (c *Client) LatestRelease(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.APIBase, c.Owner, c.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("github API %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var r Release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &r, nil
}

// AssetForCurrentPlatform picks the asset matching this binary's OS+arch
// from r.Assets. Returns an error if none match.
func AssetForCurrentPlatform(r *Release) (*Asset, error) {
	return assetFor(r, runtime.GOOS, runtime.GOARCH)
}

func assetFor(r *Release, goos, goarch string) (*Asset, error) {
	// Goreleaser convention: amd64 → x86_64; arm64 stays arm64.
	arch := goarch
	if arch == "amd64" {
		arch = "x86_64"
	}
	osNeedle := goos
	archNeedle := arch
	extSuffix := ".tar.gz"
	if goos == "windows" {
		extSuffix = ".zip"
	}
	for _, a := range r.Assets {
		n := strings.ToLower(a.Name)
		if strings.Contains(n, osNeedle) && strings.Contains(n, archNeedle) && strings.HasSuffix(n, extSuffix) {
			a := a
			return &a, nil
		}
	}
	return nil, fmt.Errorf("no asset matched %s/%s (%s) in %d candidates", goos, goarch, extSuffix, len(r.Assets))
}

// FindChecksumsAsset returns the asset that ships SHA256 checksums.
func FindChecksumsAsset(r *Release) (*Asset, error) {
	for _, a := range r.Assets {
		if strings.EqualFold(a.Name, "checksums.txt") {
			a := a
			return &a, nil
		}
	}
	return nil, errors.New("no checksums.txt in release assets")
}

// DownloadAndVerify fetches the archive and verifies its SHA256 against
// the line for asset.Name in checksums.txt. Returns the local archive path.
func (c *Client) DownloadAndVerify(ctx context.Context, asset, checksums *Asset, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	archivePath := filepath.Join(destDir, asset.Name)
	if err := c.downloadTo(ctx, asset.URL, archivePath); err != nil {
		return "", err
	}
	sumsPath := filepath.Join(destDir, "checksums.txt")
	if err := c.downloadTo(ctx, checksums.URL, sumsPath); err != nil {
		return "", err
	}
	want, err := lookupSHA256(sumsPath, asset.Name)
	if err != nil {
		return "", err
	}
	got, err := fileSHA256(archivePath)
	if err != nil {
		return "", err
	}
	if got != want {
		return "", fmt.Errorf("checksum mismatch for %s: got %s, want %s", asset.Name, got, want)
	}
	return archivePath, nil
}

func (c *Client) downloadTo(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// lookupSHA256 reads checksums.txt (lines: "<hex>  <filename>") and returns
// the hex for filename.
func lookupSHA256(checksumsFile, filename string) (string, error) {
	data, err := os.ReadFile(checksumsFile)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// goreleaser format: "<sha256>  <filename>"
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == filename {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("no checksum entry for %s in %s", filename, checksumsFile)
}

// ExtractBinary opens a tar.gz or zip archive and writes the contained `csk`
// (or `csk.exe`) binary to destPath. Returns the path written.
func ExtractBinary(archivePath, destPath string) error {
	binaryName := "csk"
	if runtime.GOOS == "windows" {
		binaryName = "csk.exe"
	}
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return extractZipBinary(archivePath, destPath, binaryName)
	}
	return extractTarGzBinary(archivePath, destPath, binaryName)
}

func extractTarGzBinary(archivePath, destPath, binaryName string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(hdr.Name) != binaryName {
			continue
		}
		return writeBinary(destPath, tr)
	}
	return fmt.Errorf("%s not found in archive", binaryName)
}

func extractZipBinary(archivePath, destPath, binaryName string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if filepath.Base(zf.Name) != binaryName {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		err = writeBinary(destPath, rc)
		_ = rc.Close()
		return err
	}
	return fmt.Errorf("%s not found in archive", binaryName)
}

func writeBinary(destPath string, src io.Reader) error {
	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// SwapBinary replaces currentExe with newBinary atomically.
//
// On Unix we just rename — open file handles on the old binary remain valid
// because the inode survives. On Windows the running .exe is locked, so we
// rename it aside (.old) first, then rename the new binary into place. The
// .old file is left for the OS to clean up on next reboot.
func SwapBinary(currentExe, newBinary string) error {
	if runtime.GOOS == "windows" {
		old := currentExe + ".old"
		_ = os.Remove(old) // tolerate "doesn't exist"
		if err := os.Rename(currentExe, old); err != nil {
			return fmt.Errorf("rename current → .old: %w", err)
		}
		if err := os.Rename(newBinary, currentExe); err != nil {
			_ = os.Rename(old, currentExe) // best effort restore
			return fmt.Errorf("rename new → current: %w", err)
		}
		return nil
	}
	return os.Rename(newBinary, currentExe)
}

// SameVersion returns true if a and b refer to the same semver tag, comparing
// after a "v" prefix is stripped from either side.
func SameVersion(a, b string) bool {
	return strings.TrimPrefix(a, "v") == strings.TrimPrefix(b, "v")
}
