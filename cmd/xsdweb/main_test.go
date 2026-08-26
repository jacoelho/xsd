package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewServerServesIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	srv := httptest.NewServer(newServer(":0", dir).Handler)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/", http.NoBody)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("Body.Close() error = %v", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q, want %q", body, "ok")
	}
}

func TestNewServerBoundsConnections(t *testing.T) {
	srv := newServer(defaultAddress, t.TempDir())
	if srv.Addr != defaultAddress {
		t.Fatalf("Addr = %q, want %q", srv.Addr, defaultAddress)
	}
	if srv.ReadHeaderTimeout != 5*time.Second || srv.ReadTimeout != 15*time.Second ||
		srv.WriteTimeout != 30*time.Second || srv.IdleTimeout != 60*time.Second {
		t.Fatalf("server timeouts = header %s, read %s, write %s, idle %s",
			srv.ReadHeaderTimeout, srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout)
	}
	if srv.MaxHeaderBytes != 16<<10 {
		t.Fatalf("MaxHeaderBytes = %d, want %d", srv.MaxHeaderBytes, 16<<10)
	}
}

func TestServeStopsCleanlyOnCancellation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var listen net.ListenConfig
	listener, err := listen.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, newServer(listener.Addr().String(), dir), listener)
	}()

	client := &http.Client{Timeout: time.Second}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+listener.Addr().String()+"/", http.NoBody)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	closeResponse(t, resp)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve() did not stop after cancellation")
	}
}

func TestValidateAssetsRequiresCompleteBuild(t *testing.T) {
	dir := t.TempDir()
	err := validateAssets(dir)
	if err == nil || !strings.Contains(err.Error(), "make wasm") {
		t.Fatalf("validateAssets() error = %v, want build instruction", err)
	}

	writeRequiredAssets(t, dir)
	if err := validateAssets(dir); err != nil {
		t.Fatalf("validateAssets() error = %v", err)
	}

	wasm := filepath.Join(dir, "xsd.wasm")
	if err := os.Remove(wasm); err != nil {
		t.Fatalf("Remove(%s) error = %v", wasm, err)
	}
	if err := os.Symlink(filepath.Join(dir, "index.html"), wasm); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	if err := validateAssets(dir); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("validateAssets(symlink) error = %v, want regular-file error", err)
	}
}

func TestAssetHandlerSetsHeadersRejectsWritesAndHidesDirectoryListings(t *testing.T) {
	dir := t.TempDir()
	writeRequiredAssets(t, dir)
	srv := httptest.NewServer(assetHandler(dir))
	defer srv.Close()

	resp := request(t, srv, http.MethodGet, "/xsd.wasm")
	if got := resp.Header.Get("Content-Type"); got != "application/wasm" {
		t.Fatalf("Content-Type = %q, want application/wasm", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	closeResponse(t, resp)

	resp = request(t, srv, http.MethodPost, "/")
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
	closeResponse(t, resp)

	resp = request(t, srv, http.MethodGet, "/js/")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("directory status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	closeResponse(t, resp)
}

func TestAssetHandlerServesOnlyCatalog(t *testing.T) {
	dir := t.TempDir()
	writeRequiredAssets(t, dir)
	for _, name := range []string{
		"package.json",
		"validation-worker.test.js",
		"node_modules/module/index.js",
		".secret",
		"browser/validator.spec.js",
	} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", path, err)
		}
		if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
	}
	if err := os.Symlink(filepath.Join(dir, "package.json"), filepath.Join(dir, "leak")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	srv := httptest.NewServer(assetHandler(dir))
	defer srv.Close()
	for _, path := range []string{
		"/index.html",
		"/package.json",
		"/validation-worker.test.js",
		"/node_modules/module/index.js",
		"/.secret",
		"/browser/validator.spec.js",
		"/leak",
	} {
		resp := request(t, srv, http.MethodGet, path)
		if resp.StatusCode != http.StatusNotFound {
			closeResponse(t, resp)
			t.Fatalf("GET %s status = %d, want %d", path, resp.StatusCode, http.StatusNotFound)
		}
		closeResponse(t, resp)
	}
}

func writeRequiredAssets(t *testing.T, dir string) {
	t.Helper()
	for _, asset := range webAssets {
		path := filepath.Join(dir, filepath.FromSlash(asset.name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", path, err)
		}
		if err := os.WriteFile(path, []byte("asset"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
	}
}

func request(t *testing.T, srv *httptest.Server, method, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+path, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	return resp
}

func closeResponse(t *testing.T, resp *http.Response) {
	t.Helper()
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("Body.Close() error = %v", err)
	}
}
