package roosterapp

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestListenFallsForwardWhenPreferredPortIsBusy(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	listener, err := listen(occupied.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if listener.Addr().String() == occupied.Addr().String() {
		t.Fatal("listener reused an occupied address")
	}
}

func TestDataDirectoryRejectsSecondInstanceAndReleasesLock(t *testing.T) {
	dir := t.TempDir()
	app, err := Start(context.Background(), dir, "127.0.0.1:0", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close(context.Background())
	second, err := Start(context.Background(), dir, "127.0.0.1:0", nil)
	if err == nil {
		second.Close(context.Background())
		t.Fatal("second instance unexpectedly opened the same data directory")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Close(ctx); err != nil {
		t.Fatal(err)
	}
	reopened, err := Start(context.Background(), dir, "127.0.0.1:0", nil)
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	defer reopened.Close(context.Background())
}
