package cr100

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

type waitingDetectionPort struct {
	entered chan struct{}
	once    sync.Once
	writes  int
}

func (p *waitingDetectionPort) Write(b []byte) (int, error) { p.writes++; return len(b), nil }
func (p *waitingDetectionPort) Read([]byte) (int, error) {
	p.once.Do(func() { close(p.entered) })
	time.Sleep(10 * time.Millisecond)
	return 0, io.EOF
}
func (*waitingDetectionPort) Close() error { return nil }

func TestDetectionShutdownInterruptsWaitingPoll(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port := &waitingDetectionPort{entered: make(chan struct{})}
	r := NewReaderWithReadRecovery(NewDevice(port))
	r.SetDetectionContext(ctx)
	done := make(chan error, 1)
	go func() { _, err := r.ConnectCard(); done <- err }()
	select {
	case <-port.entered:
	case <-time.After(time.Second):
		t.Fatal("poll did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("poll ignored shutdown")
	}
	if port.writes != 1 {
		t.Fatalf("commands after cancellation: %d", port.writes)
	}
}

func TestCancelledDetectionDoesNotCancelAPDUOrCommit(t *testing.T) {
	for _, command := range []byte{0x5a, 0x8d, 0xc7} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		port := &memoryPort{data: []byte{0x4d, 0, 7, 0x24, 0x17, 0x10, 0, 0, 1, 0, 0x68}}
		r := NewReaderWithReadRecovery(NewDevice(port))
		r.SetDetectionContext(ctx)
		_, err := (&Card{reader: r}).Apdu([]byte{command})
		if command == 0x5a {
			if !errors.Is(err, ErrEmptyAPDU) {
				t.Fatalf("APDU was cancelled: %v", err)
			}
		} else if err != nil {
			t.Fatalf("write/commit %02X changed: %v", command, err)
		}
	}
}
