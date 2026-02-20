package cr100

import (
	"io"
	"time"

	"github.com/dumacp/go-dspread/internal/device"
)

type Device struct {
	dev *device.Device
}

func NewDevice(rw io.ReadWriteCloser) *Device {
	return &Device{
		dev: device.NewDevice(rw),
	}
}

func (d *Device) Transmit(in []byte, timeout time.Duration) ([]byte, error) {
	return d.dev.Transmit(in, timeout)
}

func (d *Device) Close() error {
	return d.dev.Close()
}
