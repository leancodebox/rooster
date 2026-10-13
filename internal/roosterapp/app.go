package roosterapp

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/leancodebox/rooster/internal/engine"
	"github.com/leancodebox/rooster/internal/httpapi"
	"github.com/leancodebox/rooster/internal/runner"
	storesqlite "github.com/leancodebox/rooster/internal/store/sqlite"
)

type Application struct {
	store        *storesqlite.Store
	engine       *engine.Engine
	server       *httpapi.Server
	listener     net.Listener
	serveErr     chan error
	closeOnce    sync.Once
	instanceLock *os.File
	closeDone    chan struct{}
	closeErr     error
}

func Start(ctx context.Context, dataDir, address string, assets fs.FS) (*Application, error) {
	limits, err := executionLimits()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "logs"), 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	instanceLock, err := os.OpenFile(filepath.Join(dataDir, "rooster.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open instance lock: %w", err)
	}
	if err := lockFile(instanceLock); err != nil {
		instanceLock.Close()
		return nil, fmt.Errorf("data directory already in use or cannot be locked: %w", err)
	}
	started := false
	defer func() {
		if !started {
			instanceLock.Close()
		}
	}()
	st, err := storesqlite.Open(filepath.Join(dataDir, "rooster.db"))
	if err != nil {
		return nil, err
	}
	environment := runner.NewEnvironmentProvider()
	refreshCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	if err := environment.Refresh(refreshCtx, ""); err != nil {
		slog.Warn("using inherited environment", "error", err)
	}
	cancel()
	eng := engine.New(st, runner.New(environment), filepath.Join(dataDir, "logs"), limits)
	if err := eng.Start(ctx); err != nil {
		st.Close()
		return nil, err
	}
	listener, err := listen(address)
	if err != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = eng.Close(closeCtx)
		st.Close()
		return nil, err
	}
	server := httpapi.New(eng, assets)
	app := &Application{store: st, engine: eng, server: server, listener: listener, serveErr: make(chan error, 1)}
	app.instanceLock = instanceLock
	app.closeDone = make(chan struct{})
	started = true
	go func() { app.serveErr <- server.Serve(listener) }()
	slog.Info("rooster ready", "url", app.URL(), "data", dataDir)
	return app, nil
}

func listen(address string) (net.Listener, error) {
	listener, err := net.Listen("tcp", address)
	if err == nil {
		return listener, nil
	}
	host, portText, splitErr := net.SplitHostPort(address)
	if splitErr != nil {
		return nil, err
	}
	port, parseErr := strconv.Atoi(portText)
	if parseErr != nil || port == 0 {
		return nil, err
	}
	for offset := 1; offset <= 100; offset++ {
		candidate := net.JoinHostPort(host, strconv.Itoa(port+offset))
		listener, listenErr := net.Listen("tcp", candidate)
		if listenErr == nil {
			return listener, nil
		}
	}
	return nil, fmt.Errorf("listen on %s or the next 100 ports: %w", address, err)
}

func (a *Application) URL() string              { return "http://" + a.listener.Addr().String() }
func (a *Application) ServeError() <-chan error { return a.serveErr }

func (a *Application) Close(ctx context.Context) error {
	a.closeOnce.Do(func() {
		go func() {
			defer close(a.closeDone)
			// Keep the database and instance lock until all executions are reaped,
			// even if a caller stops waiting for graceful shutdown.
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := a.server.Shutdown(shutdownCtx); err != nil {
				a.closeErr = err
				_ = a.server.Shutdown(context.Background())
			}
			if err := a.engine.Close(context.Background()); err != nil && a.closeErr == nil {
				a.closeErr = err
			}
			if err := a.store.Close(); err != nil && a.closeErr == nil {
				a.closeErr = err
			}
			if err := a.instanceLock.Close(); err != nil && a.closeErr == nil {
				a.closeErr = err
			}
		}()
	})
	select {
	case <-a.closeDone:
		return a.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func IsExpectedServerClose(err error) bool { return err == nil || err == http.ErrServerClosed }
