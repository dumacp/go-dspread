package cr100

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/dumacp/go-dspread/internal/cgo"
)

func TestFailedPollDoesNotConsultCachedSDKStatus(t *testing.T) {
	checked := false
	err := boundedReadExchange([]byte{1}, true, time.Second,
		func([]byte, time.Duration) ([]byte, error) { return nil, io.ErrUnexpectedEOF },
		func() (int, error) { checked = true; return cgo.CMD_SUC, nil },
		func() ([]byte, error) { t.Fatal("query after failed poll"); return nil, nil })
	var retry *RetryableReadError
	if checked || !errors.Is(err, io.ErrUnexpectedEOF) || !errors.As(err, &retry) {
		t.Fatalf("consulted SDK=%v error=%v", checked, err)
	}
}

func TestBusyAndDetectionContinuationHaveTotalDeadline(t *testing.T) {
	for _, code := range []int{0, cgo.CMD_BUSY, cgo.CMD_CONTINUE} {
		calls := 0
		start := time.Now()
		err := boundedReadExchange([]byte{1}, true, 35*time.Millisecond,
			func(_ []byte, timeout time.Duration) ([]byte, error) {
				calls++
				if timeout > 35*time.Millisecond {
					t.Fatalf("timeout exceeds budget: %v", timeout)
				}
				return nil, nil
			},
			func() (int, error) { return code, nil }, func() ([]byte, error) { return []byte{2}, nil })
		elapsed := time.Since(start)
		if !errors.Is(err, context.DeadlineExceeded) || calls > 6 || elapsed > 300*time.Millisecond {
			t.Fatalf("status %X calls=%d elapsed=%v err=%v", code, calls, elapsed, err)
		}
	}
}

func TestBusyThenSuccessRequiresFreshExchange(t *testing.T) {
	calls := 0
	err := boundedReadExchange([]byte{1}, false, time.Second,
		func(cmd []byte, _ time.Duration) ([]byte, error) {
			calls++
			if calls == 2 && cmd[0] != 2 {
				t.Fatal("missing query")
			}
			return nil, nil
		},
		func() (int, error) {
			if calls == 1 {
				return cgo.CMD_BUSY, nil
			}
			return cgo.CMD_SUC, nil
		},
		func() ([]byte, error) { return []byte{2}, nil })
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestTerminalSDKRejectionIsNotRetried(t *testing.T) {
	err := boundedReadExchange([]byte{1}, true, time.Second,
		func([]byte, time.Duration) ([]byte, error) { return nil, nil },
		func() (int, error) { return cgo.CMD_DECLINE, nil }, func() ([]byte, error) { t.Fatal("query on rejection"); return nil, nil })
	var retry *RetryableReadError
	if err == nil || errors.As(err, &retry) {
		t.Fatalf("unexpected classification: %v", err)
	}
}

func TestStatusFailureAndLateSuccessCannotUsePayload(t *testing.T) {
	for _, late := range []bool{false, true} {
		err := boundedReadExchange([]byte{1}, false, 15*time.Millisecond,
			func([]byte, time.Duration) ([]byte, error) {
				if late {
					time.Sleep(20 * time.Millisecond)
				}
				return nil, nil
			},
			func() (int, error) {
				if late {
					t.Fatal("status consulted after deadline")
				}
				return 0, io.ErrUnexpectedEOF
			},
			func() ([]byte, error) { return nil, nil })
		var retry *RetryableReadError
		if !errors.As(err, &retry) {
			t.Fatalf("late=%v err=%v", late, err)
		}
	}
}

// A real ConnectCard call must propagate transport failure rather than reading
// UID fields left in the process-global vendor SDK by a previous card.
type failedPollPort struct{ reads int }

func (p *failedPollPort) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func (p *failedPollPort) Read([]byte) (int, error)  { p.reads++; return 0, io.EOF }
func (p *failedPollPort) Close() error              { return nil }
func TestConnectCardRejectsFailedPoll(t *testing.T) {
	p := &failedPollPort{}
	r := NewReaderWithReadRecovery(NewDevice(p))
	card, err := r.ConnectCard()
	var retry *RetryableReadError
	if card != nil || !errors.As(err, &retry) || p.reads != 0 {
		t.Fatalf("card=%v err=%v reads=%d", card, err, p.reads)
	}
}

func TestCompleteDetectionFrameSuppliesUID(t *testing.T) {
	p := &memoryPort{data: []byte{0x4d, 0, 0x1a, 0x24, 0x17, 0x80, 0, 0, 0x14, 1, 0, 2, 0x44, 3, 0x20, 7, 4, 0x4c, 0x5c, 0x92, 0xe8, 0x62, 0x80, 5, 0x75, 0x77, 0x81, 2, 0x80, 0x1b}}
	r := NewReaderWithReadRecovery(NewDevice(p))
	card, err := r.ConnectCard()
	if err != nil {
		t.Fatal(err)
	}
	uid, err := card.UID()
	if err != nil || len(uid) != 7 || uid[0] != 4 {
		t.Fatalf("uid=%X err=%v", uid, err)
	}
}
