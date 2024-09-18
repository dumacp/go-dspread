package cr100

import (
	"context"
	"io"

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

func (d *Device) Transmit(in []byte, ctx context.Context) ([]byte, error) {
	return d.dev.Transmit(in, ctx)
}

func (d *Device) Close() error {
	return d.dev.Close()
}
