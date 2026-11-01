package cr100

import (
	"errors"
	"fmt"
	"github.com/dumacp/smartcard"
	"time"
)

var ErrEmptyAPDU = errors.New("CR100 returned an empty APDU payload")

// RetryableReadError marks a failed exchange, not a card's explicit rejection.
// Consumers may restart a read-only session; writes still require reconciliation.
type RetryableReadError struct{ Err error }

func (e *RetryableReadError) Error() string       { return e.Err.Error() }
func (e *RetryableReadError) Unwrap() error       { return e.Err }
func (e *RetryableReadError) RetryCardRead() bool { return true }

// NewReaderWithReadRecovery opts this reader into fresh-session read retries.
// NewReaderWithDevice keeps the historical behavior.
func NewReaderWithReadRecovery(dev *Device) *Reader {
	r := NewReaderWithDevice(dev)
	r.EnableReadRecovery()
	return r
}

// EnableReadRecovery must be called before this reader is used by an actor.
func (r *Reader) EnableReadRecovery() { r.readRecovery = true }

func (r *Reader) ReadRecoveryEnabled() bool { return r.readRecovery }

func (r *Reader) ReadRetryDelay(err error) (time.Duration, bool) {
	if !r.readRecovery {
		return 0, false
	}
	var retryable interface{ RetryCardRead() bool }
	if errors.Is(err, smartcard.ErrNoSmartcard) || (errors.As(err, &retryable) && retryable.RetryCardRead()) {
		return 150 * time.Millisecond, true
	}
	return 0, false
}
func recoveryReadCommand(apdu []byte) bool {
	if len(apdu) == 0 {
		return false
	}
	switch apdu[0] {
	case 0x5a, 0x71, 0xaf, 0xad:
		return true
	}
	return false
}
func (r *Reader) readExchangeError(apdu []byte, err error) error {
	if r.readRecovery && recoveryReadCommand(apdu) {
		return &RetryableReadError{Err: fmt.Errorf("CR100 exchange: %w", err)}
	}
	return err
}
