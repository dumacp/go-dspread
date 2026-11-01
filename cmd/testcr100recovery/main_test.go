package main

import (
	"github.com/dumacp/smartcard"
	"testing"
)

type fakeCard struct {
	smartcard.ICard
	calls    int
	response []byte
}

func (c *fakeCard) Apdu([]byte) ([]byte, error) { c.calls++; return c.response, nil }

func TestRejectWritesBeforeTransport(t *testing.T) {
	for _, cmd := range []byte{0x8d, 0x8b, 0xc7, 0x3d, 0x0c} {
		transport := &fakeCard{}
		_, err := (readOnlyCard{transport}).Apdu([]byte{cmd})
		if err == nil || transport.calls != 0 {
			t.Fatalf("write %02X reached transport or was accepted", cmd)
		}
	}
}
func TestEmptyAPDUIsTransportFailure(t *testing.T) {
	transport := &fakeCard{}
	_, err := (readOnlyCard{transport}).Apdu([]byte{0x71, 4, 0})
	if err == nil || transport.calls != 1 {
		t.Fatal("empty APDU accepted as success")
	}
}
