package cr100

import smartcard "github.com/dumacp/smartcard"

type Reader struct {
	smartcard.IReader
	dev *Device
}

func NewReaderWithDevice(dev *Device) *Reader {
	return &Reader{
		dev: dev,
	}
}

// ConnectCard connect card with protocol T=1
func (r *Reader) ConnectCard() (smartcard.ICard, error) {
	return nil, nil
}

// ConnectCard connect card with protocol T=1.
// Some readers distinguish between the flow to connect a contact-based smart card and a contactless smart card.
func (r *Reader) ConnectSamCard() (smartcard.ICard, error) {
	panic("not implemented") // TODO: Implement
}

// ConnectSamCard_T0 ConnectCard connect card with protocol T=1.
func (r *Reader) ConnectSamCard_T0() (smartcard.ICard, error) {
	panic("not implemented") // TODO: Implement
}

// ConnectSamCard_Tany ConnectCard connect card with protocol T=any.
func (r *Reader) ConnectSamCard_Tany() (smartcard.ICard, error) {
	panic("not implemented") // TODO: Implement
}
