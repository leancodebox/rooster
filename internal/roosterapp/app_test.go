package roosterapp

import (
	"net"
	"testing"
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
