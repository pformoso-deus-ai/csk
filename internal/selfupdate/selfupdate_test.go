package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeRelease spins up an httptest server that serves a /repos/.../latest
// endpoint plus an asset URL and a checksums.txt URL. Returns the assembled
// Client and a teardown.
func fakeRelease(t *testing.T, tag string, assetName string, binaryContent []byte) (*Client, *Release) {
	t.Helper()

	// Build the tar.gz containing one file named "csk" (or csk.exe).
	binaryName := "csk"
	if strings.HasSuffix(assetName, ".zip") {
		binaryName = "csk.exe"
	}
	var archive bytes.Buffer
	if strings.HasSuffix(assetName, ".zip") {
		zw := zip.NewWriter(&archive)
		w, err := zw.Create(binaryName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(binaryContent); err != nil {
			t.Fatal(err)
		}
		_ = zw.Close()
	} else {
		gz := gzip.NewWriter(&archive)
		tw := tar.NewWriter(gz)
		if err := tw.WriteHeader(&tar.Header{Name: binaryName, Mode: 0o755, Size: int64(len(binaryContent))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(binaryContent); err != nil {
			t.Fatal(err)
		}
		_ = tw.Close()
		_ = gz.Close()
	}
	archiveBytes := archive.Bytes()

	h := sha256.Sum256(archiveBytes)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(h[:]), assetName)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	assetURL := srv.URL + "/asset/" + assetName
	sumsURL := srv.URL + "/asset/checksums.txt"

	mux.HandleFunc("/asset/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			_, _ = w.Write([]byte(checksums))
		case strings.HasSuffix(r.URL.Path, "/"+assetName):
			_, _ = w.Write(archiveBytes)
		default:
			http.NotFound(w, r)
		}
	})

	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		body := fmt.Sprintf(`{
			"tag_name": %q,
			"assets": [
				{"name": %q, "browser_download_url": %q, "size": %d},
				{"name": "checksums.txt", "browser_download_url": %q, "size": %d}
			]
		}`, tag, assetName, assetURL, len(archiveBytes), sumsURL, len(checksums))
		_, _ = w.Write([]byte(body))
	})

	c := &Client{
		HTTP:    srv.Client(),
		APIBase: srv.URL,
		Owner:   "test",
		Repo:    "csk",
	}
	rel := &Release{
		TagName: tag,
		Assets: []Asset{
			{Name: assetName, URL: assetURL, Size: int64(len(archiveBytes))},
			{Name: "checksums.txt", URL: sumsURL, Size: int64(len(checksums))},
		},
	}
	return c, rel
}

func TestLatestRelease(t *testing.T) {
	c, _ := fakeRelease(t, "v0.9.9", "csk_0.9.9_linux_x86_64.tar.gz", []byte("fake binary"))
	rel, err := c.LatestRelease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.TagName != "v0.9.9" {
		t.Errorf("tag = %q", rel.TagName)
	}
	if len(rel.Assets) != 2 {
		t.Errorf("assets = %d", len(rel.Assets))
	}
}

func TestAssetFor(t *testing.T) {
	r := &Release{Assets: []Asset{
		{Name: "csk_0.1.0_linux_x86_64.tar.gz"},
		{Name: "csk_0.1.0_linux_arm64.tar.gz"},
		{Name: "csk_0.1.0_darwin_x86_64.tar.gz"},
		{Name: "csk_0.1.0_darwin_arm64.tar.gz"},
		{Name: "csk_0.1.0_windows_x86_64.zip"},
		{Name: "csk_0.1.0_windows_arm64.zip"},
	}}
	cases := []struct{ os, arch, want string }{
		{"linux", "amd64", "csk_0.1.0_linux_x86_64.tar.gz"},
		{"linux", "arm64", "csk_0.1.0_linux_arm64.tar.gz"},
		{"darwin", "amd64", "csk_0.1.0_darwin_x86_64.tar.gz"},
		{"darwin", "arm64", "csk_0.1.0_darwin_arm64.tar.gz"},
		{"windows", "amd64", "csk_0.1.0_windows_x86_64.zip"},
	}
	for _, c := range cases {
		got, err := assetFor(r, c.os, c.arch)
		if err != nil {
			t.Errorf("%s/%s: %v", c.os, c.arch, err)
			continue
		}
		if got.Name != c.want {
			t.Errorf("%s/%s = %q, want %q", c.os, c.arch, got.Name, c.want)
		}
	}
}

func TestAssetFor_NoMatch(t *testing.T) {
	r := &Release{Assets: []Asset{{Name: "csk_0.1.0_linux_x86_64.tar.gz"}}}
	if _, err := assetFor(r, "plan9", "amd64", ); err == nil {
		t.Error("expected no-match error")
	}
}

func TestFindChecksumsAsset(t *testing.T) {
	r := &Release{Assets: []Asset{{Name: "checksums.txt"}, {Name: "other.txt"}}}
	a, err := FindChecksumsAsset(r)
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "checksums.txt" {
		t.Errorf("got %q", a.Name)
	}
	r2 := &Release{Assets: []Asset{{Name: "other.txt"}}}
	if _, err := FindChecksumsAsset(r2); err == nil {
		t.Error("expected missing-checksums error")
	}
}

func TestDownloadAndVerify(t *testing.T) {
	platformAsset := "csk_0.9.9_linux_x86_64.tar.gz"
	c, rel := fakeRelease(t, "v0.9.9", platformAsset, []byte("hello world"))
	asset := &rel.Assets[0]
	sums := &rel.Assets[1]

	dir := t.TempDir()
	archive, err := c.DownloadAndVerify(context.Background(), asset, sums, dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(archive) != platformAsset {
		t.Errorf("archive name = %q", filepath.Base(archive))
	}
}

func TestDownloadAndVerify_ChecksumMismatch(t *testing.T) {
	// Build a release whose checksums.txt lies.
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/asset/csk_0.0.0_linux_x86_64.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake archive contents"))
	})
	mux.HandleFunc("/asset/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		// Wrong sha for the archive.
		_, _ = w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000  csk_0.0.0_linux_x86_64.tar.gz\n"))
	})
	c := &Client{HTTP: srv.Client(), APIBase: srv.URL, Owner: "x", Repo: "y"}
	asset := &Asset{Name: "csk_0.0.0_linux_x86_64.tar.gz", URL: srv.URL + "/asset/csk_0.0.0_linux_x86_64.tar.gz"}
	sums := &Asset{Name: "checksums.txt", URL: srv.URL + "/asset/checksums.txt"}
	_, err := c.DownloadAndVerify(context.Background(), asset, sums, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected checksum-mismatch error, got %v", err)
	}
}

func TestExtractBinary_TarGz(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tar.gz binary extraction path tests the unix flow")
	}
	c, rel := fakeRelease(t, "v1.0.0", "csk_1.0.0_linux_x86_64.tar.gz", []byte("#!/bin/sh\necho hi\n"))
	dir := t.TempDir()
	archive, err := c.DownloadAndVerify(context.Background(), &rel.Assets[0], &rel.Assets[1], dir)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "csk")
	if err := ExtractBinary(archive, dest); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(dest)
	if !strings.Contains(string(body), "echo hi") {
		t.Errorf("extracted body = %q", body)
	}
}

func TestExtractBinary_Zip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("zip path is exercised only on windows targets")
	}
	c, rel := fakeRelease(t, "v1.0.0", "csk_1.0.0_windows_x86_64.zip", []byte("fake.exe"))
	dir := t.TempDir()
	archive, err := c.DownloadAndVerify(context.Background(), &rel.Assets[0], &rel.Assets[1], dir)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "csk.exe")
	if err := ExtractBinary(archive, dest); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(dest)
	if string(body) != "fake.exe" {
		t.Errorf("extracted body = %q", body)
	}
}

func TestSwapBinary(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "csk")
	if runtime.GOOS == "windows" {
		current = filepath.Join(dir, "csk.exe")
	}
	if err := os.WriteFile(current, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	newBin := filepath.Join(dir, "csk-new")
	if err := os.WriteFile(newBin, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SwapBinary(current, newBin); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(current)
	if string(body) != "new" {
		t.Errorf("current = %q, want %q", body, "new")
	}
}

func TestSameVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v0.1.0", "0.1.0", true},
		{"v0.1.0", "v0.1.0", true},
		{"0.1.0", "0.1.0", true},
		{"v0.1.0", "v0.1.1", false},
		{"", "", true},
	}
	for _, c := range cases {
		if got := SameVersion(c.a, c.b); got != c.want {
			t.Errorf("SameVersion(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
