package cr100

import (
	"context"
	"fmt"
	"time"

	"github.com/dumacp/go-dspread/internal/cgo"
)

const (
	readExchangeBudget = time.Second
	// The existing DoMifare(1, 20) scan takes about 20s with no card on
	// the tested firmware. Allow it to finish before issuing another scan.
	detectionExchangeBudget = 22 * time.Second
)

// The serial port must also have a finite ReadTimeout: Transmit cannot interrupt
// a blocking port Read. The opted-in appfare CR100 factory sets it to 30ms.
func (r *Reader) exchangeRead(command []byte, detection bool) error {
	budget := readExchangeBudget
	if detection {
		budget = detectionExchangeBudget
	}
	transmit := r.dev.Transmit
	if detection && r.detectionContext != nil {
		transmit = func(command []byte, timeout time.Duration) ([]byte, error) {
			return r.dev.TransmitContext(r.detectionContext, command, timeout)
		}
	}
	return boundedReadExchange(command, detection, budget,
		transmit, cgo.GetCmdId, cgo.PackQueryLatestCmdResult)
}

// Never inspect cached SDK fields after an incomplete transport exchange. A
// successful terminal SDK status is required before the caller can read a UID
// or APDU payload. This helper is only used by opted-in read-only exchanges.
func boundedReadExchange(command []byte, detection bool, budget time.Duration,
	transmit func([]byte, time.Duration) ([]byte, error),
	status func() (int, error), query func() ([]byte, error)) error {
	deadline := time.Now().Add(budget)
	retryable := func(err error) error { return &RetryableReadError{Err: err} }
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return retryable(fmt.Errorf("CR100 exchange deadline: %w", context.DeadlineExceeded))
		}
		perTransmit := 300 * time.Millisecond
		if detection {
			perTransmit = detectionExchangeBudget
		}
		timeout := min(perTransmit, remaining)
		if _, err := transmit(command, timeout); err != nil {
			return retryable(fmt.Errorf("CR100 exchange transport: %w", err))
		}
		if time.Until(deadline) <= 0 {
			return retryable(fmt.Errorf("CR100 exchange deadline: %w", context.DeadlineExceeded))
		}
		code, err := status()
		if err != nil {
			return retryable(fmt.Errorf("CR100 exchange status: %w", err))
		}
		switch code {
		case cgo.CMD_SUC:
			return nil
		case cgo.CMD_BUSY, cgo.CMD_CONTINUE:
			command, err = query()
			if err != nil {
				return fmt.Errorf("CR100 packing status query: %w", err)
			}
		case 0:
			if !detection {
				return retryable(fmt.Errorf("CR100 empty command status"))
			}
			// Preserve the SDK's existing detection continuation command.
			command = []byte{0}
		case cgo.CMD_TIMEOUT, cgo.CMD_CANCEL:
			return retryable(fmt.Errorf("CR100 exchange status 0x%X", code))
		default:
			// An explicit terminal rejection is not a communication retry.
			return fmt.Errorf("CR100 exchange rejected: status 0x%X", code)
		}
		remaining = time.Until(deadline)
		if remaining <= 0 {
			return retryable(fmt.Errorf("CR100 exchange deadline: %w", context.DeadlineExceeded))
		}
		// Pace BUSY/CONTINUE even if the transport responds immediately.
		time.Sleep(min(10*time.Millisecond, remaining))
	}
}
