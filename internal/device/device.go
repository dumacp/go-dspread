package device

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dumacp/go-dspread/internal/cgo"
	"github.com/dumacp/smartcard"
)

type Device struct {
	readerWriter io.ReadWriteCloser
}

func NewDevice(rw io.ReadWriteCloser) *Device {
	return &Device{readerWriter: rw}
}

func (d *Device) Transmit(in []byte, timeout time.Duration) ([]byte, error) {

	res := make([]byte, 0)
	buff := make([]byte, 1024)
	fmt.Printf("apdu to send: 0x%X\n", in)
	if _, err := d.readerWriter.Write(in); err != nil {
		// return nil, fmt.Errorf("error writing to serial port: %v", err)
		return nil, fmt.Errorf("error writing to serial port: %v (%w)", err, smartcard.ErrComm)
	}
	fmt.Printf("apdu sent: 0x%X\n", in)

	tick0 := time.NewTimer(6 * time.Millisecond)
	defer tick0.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	funcRead := func() ([]byte, error) {
		t0 := time.Now()
		n, err := d.readerWriter.Read(buff)
		if err != nil {
			switch {
			case n > 0 && errors.Is(err, io.EOF):
				// fmt.Printf("error reading from serial port (n = %d): %v\n", n, err)
			case n > 0:
				// fmt.Printf("error reading from serial port (n = %d): %v\n", n, err)
			case errors.Is(err, io.EOF):
				// fmt.Printf("error reading from serial port: %v\n", err)
				if time.Since(t0) < 5*time.Millisecond {
					return nil, fmt.Errorf("error reading from serial port (<10ms): %w", err)
				}
				return nil, nil
			default:
				return nil, fmt.Errorf("error reading from serial port: %w", err)
			}
		}
		res = append(res, buff[0:n]...)
		// for {
		// 	n3, err := s.Read(buff)
		// 	if err != nil {
		// 		break
		// 	}
		// 	res = append(res, buff[0:n3]...)
		// }

		fmt.Printf("Response transmit: %X\n", res)

		t1 := time.Now()

		if _, err := cgo.OnPackage(res); err != nil {
			return nil, fmt.Errorf("error on package: %v", err)
		}
		fmt.Printf("time onpackage: %v\n", time.Since(t1).Milliseconds())
		return res, nil
	}

	for {

		select {
		case <-tick0.C:
			if out, err := funcRead(); err != nil {
				fmt.Printf("error on transmit: %v\n", err)
				return nil, err
			} else if len(out) > 0 {
				fmt.Printf("response transmit: %X\n", out)
				return out, nil
			}
		case <-tick.C:
			if out, err := funcRead(); err != nil {
				fmt.Printf("error on transmit: %v\n", err)
				return nil, err
			} else if len(out) > 0 {
				fmt.Printf("response transmit: %X\n", out)
				return out, nil
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("error on transmit, context done")
		}
	}
}

func (d *Device) Close() error {
	return d.readerWriter.Close()
}
