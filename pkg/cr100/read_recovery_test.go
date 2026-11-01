package cr100

import (
	"errors"
	"github.com/dumacp/smartcard"
	"io"
	"testing"
)

type memoryPort struct{ data []byte }

func (p *memoryPort) Write(b []byte) (int, error) { return len(b), nil }
func (p *memoryPort) Read(b []byte) (int, error) {
	if len(p.data) == 0 {
		return 0, io.EOF
	}
	n := copy(b, p.data)
	p.data = p.data[n:]
	return n, nil
}
func (p *memoryPort) Close() error { return nil }

func TestReadRecoveryOptIn(t *testing.T) {
	cases := []struct {
		name string
		r    *Reader
		err  error
		want bool
	}{
		{"default_empty", NewReaderWithDevice(nil), &RetryableReadError{Err: ErrEmptyAPDU}, false},
		{"enabled_empty", NewReaderWithReadRecovery(nil), fmtWrappedEmpty(), true},
		{"enabled_absent", NewReaderWithReadRecovery(nil), smartcard.ErrNoSmartcard, true},
		{"explicit_rejection", NewReaderWithReadRecovery(nil), errors.New("authentication rejected"), false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			delay, got := tt.r.ReadRetryDelay(tt.err)
			if got != tt.want {
				t.Fatalf("retry=%v want=%v", got, tt.want)
			}
			if got && delay <= 0 {
				t.Fatal("retry must be paced")
			}
		})
	}
}
func fmtWrappedEmpty() error {
	return errors.Join(errors.New("ReadData"), &RetryableReadError{Err: ErrEmptyAPDU})
}

func TestEmptyFrameOnlyBecomesTypedErrorWhenEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		port := &memoryPort{data: []byte{0x4d, 0, 7, 0x24, 0x17, 0x10, 0, 0, 1, 0, 0x68}}
		r := NewReaderWithDevice(NewDevice(port))
		r.readRecovery = enabled
		response, err := (&Card{reader: r}).Apdu([]byte{0x5a, 1, 0, 0})
		if enabled {
			if !errors.Is(err, ErrEmptyAPDU) {
				t.Fatalf("expected typed empty payload, got %v", err)
			}
		} else if err != nil || len(response) != 0 {
			t.Fatalf("historical result changed: %X %v", response, err)
		}
	}
}

func TestWriteCommandsDoNotAcquireReadRecoverySemantics(t *testing.T) {
	for _, cmd := range []byte{0x8d, 0x8b, 0xc7} {
		port := &memoryPort{data: []byte{0x4d, 0, 7, 0x24, 0x17, 0x10, 0, 0, 1, 0, 0x68}}
		r := NewReaderWithReadRecovery(NewDevice(port))
		data, err := (&Card{reader: r}).Apdu([]byte{cmd})
		if err != nil || len(data) != 0 {
			t.Fatalf("write %02X behavior changed: %X %v", cmd, data, err)
		}
	}
}
