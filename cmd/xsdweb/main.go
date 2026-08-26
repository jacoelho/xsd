// Package main serves local XSD web assets.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const (
	defaultAddress        = "127.0.0.1:8765"
	javascriptContentType = "text/javascript; charset=utf-8"
)

type webAsset struct {
	route       string
	name        string
	contentType string
}

var webAssets = [...]webAsset{
	{route: "/", name: "index.html", contentType: "text/html; charset=utf-8"},
	{route: "/wasm_exec.js", name: "wasm_exec.js", contentType: javascriptContentType},
	{route: "/xsd.wasm", name: "xsd.wasm", contentType: "application/wasm"},
	{route: "/js/validation-flow.js", name: "js/validation-flow.js", contentType: javascriptContentType},
	{route: "/js/validation-worker.js", name: "js/validation-worker.js", contentType: javascriptContentType},
	{route: "/js/xsd-worker.js", name: "js/xsd-worker.js", contentType: javascriptContentType},
}

func main() {
	if err := command(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func command() error {
	addr := flag.String("addr", defaultAddress, "listen address")
	dir := flag.String("dir", "docs", "directory to serve")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, *addr, *dir)
}

func run(ctx context.Context, addr, dir string) error {
	if err := validateAssets(dir); err != nil {
		return err
	}

	srv := newServer(addr, dir)
	var listen net.ListenConfig
	listener, err := listen.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	log.Printf("serving %s on %s", dir, addr)
	return serve(ctx, srv, listener)
}

func serve(ctx context.Context, srv *http.Server, listener net.Listener) error {
	result := make(chan error, 1)
	go func() {
		result <- srv.Serve(listener)
	}()

	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return errors.Join(fmt.Errorf("shut down web server: %w", err), srv.Close())
		}
		err := <-result
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func newServer(addr, dir string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           assetHandler(dir),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
}

func validateAssets(dir string) error {
	for _, asset := range webAssets {
		path := filepath.Join(dir, filepath.FromSlash(asset.name))
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("web asset %s is unavailable; run `make wasm`: %w", path, err)
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("web asset %s must be a non-empty regular file; run `make wasm`", path)
		}
	}
	return nil
}

func assetHandler(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		asset, ok := assetForRoute(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", asset.contentType)
		http.ServeFile(w, r, filepath.Join(dir, filepath.FromSlash(asset.name)))
	})
}

func assetForRoute(route string) (webAsset, bool) {
	for _, asset := range webAssets {
		if route == asset.route {
			return asset, true
		}
	}
	return webAsset{}, false
}
