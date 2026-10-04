// Command staticweb serves public releases and has no database or login dependency.
package main

import (
	"context"
	"errors"
	"flag"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/staticweb"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	root := flag.String("root", "../.runtime/publication", "publication runtime directory")
	address := flag.String("address", ":8081", "listen address")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	server := &http.Server{Addr: *address, Handler: staticweb.Handler{Root: *root}, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		_ = server.Shutdown(shutdown)
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("static web failed", "error", err)
		os.Exit(1)
	}
}
