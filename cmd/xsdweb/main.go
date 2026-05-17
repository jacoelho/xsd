// Package main serves local XSD web assets.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"
)

func main() {
	addr := flag.String("addr", ":8765", "listen address")
	dir := flag.String("dir", "docs", "directory to serve")
	flag.Parse()

	srv := &http.Server{
		Addr:              *addr,
		Handler:           http.FileServer(http.Dir(*dir)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("serving %s on %s", *dir, *addr)
	log.Fatal(srv.ListenAndServe())
}
