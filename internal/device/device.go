package device

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dumacp/go-dspread/internal/cgo"
)

type Device struct {
	readerWriter io.ReadWriteCloser
}

func NewDevice(rw io.ReadWriteCloser) *Device {
	return &Device{readerWriter: rw}
}

func (d *Device) Transmit(in []byte, ctx context.Context) ([]byte, error) {
	res := make([]byte, 0)
	buff := make([]byte, 256)
	fmt.Printf("apdu to send: 0x%X\n", in)
	if _, err := d.readerWriter.Write(in); err != nil {
		return nil, fmt.Errorf("error writing to serial port: %v", err)
	}

	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for {

		select {

		case <-tick.C:
			t0 := time.Now()
			n, err := d.readerWriter.Read(buff)
			if err != nil {
				switch {
				case n > 0 && errors.Is(err, io.EOF):
				case n > 0:
					fmt.Printf("error reading from serial port: %v\n", err)
				case errors.Is(err, io.EOF):
					fmt.Printf("error reading from serial port: %v\n", err)
					if time.Since(t0) < 10*time.Millisecond {
						return nil, fmt.Errorf("error reading from serial port: %v", err)
					}
				default:
					return nil, fmt.Errorf("error reading from serial port: %v", err)
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
		case <-ctx.Done():
			return nil, fmt.Errorf("error on transmit, context done")
		}
	}
}

func (d *Device) Close() error {
	return d.readerWriter.Close()
}
