package main

import (
	"io"
	"time"

	"github.com/dumacp/go-dspread/pkg/cr100"
)

// The caller owns the port. Its ReadTimeout must be finite so the transport's
// deadline can be observed between reads.
type borrowedPort struct{ io.ReadWriter }

func (borrowedPort) Close() error { return nil }

func Transmit(port io.ReadWriter, command []byte) ([]byte, error) {
	return cr100.NewDevice(borrowedPort{port}).Transmit(command, 300*time.Millisecond)
}
