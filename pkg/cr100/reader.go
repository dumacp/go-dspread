package cr100

import (
	"fmt"
	"log"
	"time"

	"github.com/dumacp/go-dspread/internal/cgo"
	smartcard "github.com/dumacp/smartcard"
)

type Reader struct {
	smartcard.IReader
	dev *Device
}

func NewReaderWithDevice(dev *Device) *Reader {
	return &Reader{
		dev: dev,
	}
}

func (r *Reader) Name() string {
	return "CR100"
}

// ConnectCard connect card with protocol T=1
func (r *Reader) ConnectCard() (smartcard.ICard, error) {
	poll, err := cgo.DoMifare(0x01, time.Millisecond*20)
	if err != nil {
		return nil, fmt.Errorf("error doing mifare poll: %w", err)
	}

	// ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	// defer cancel()
	if _, err := r.dev.Transmit(poll, 1000*time.Millisecond); err != nil {
		fmt.Printf("error transmiting poll: %s\n", err)
	}

	cmdid := func() int {
		c, err := cgo.GetCmdId()
		if err != nil {
			log.Printf("error reading from nfc GetCmdId: %v", err)
			return -1
		}
		fmt.Printf("function cmdid: 0x%X\n", c)
		return c
	}

	// while (getCmdId()==0x00) {

	// 	transmit(0x00,1);
	// 	sleep(1);
	//   }
	errorCounter := 0
	for cmdid() == 0x00 {
		if _, err := r.dev.Transmit([]byte{0x00}, 1000*time.Millisecond); err != nil {
			if errorCounter > 30 {
				return nil, fmt.Errorf("error transmiting poll: %w", err)
			} else {
				fmt.Printf("error transmiting poll: %s\n", err)
			}
			errorCounter++
		}
		// time.Sleep(10 * time.Millisecond)
	}

	// log.Printf("mifare SAK: %X\n", func() []byte {

	// 	s, _ := cgo.Get("mifare_SAK")
	// 	return s
	// }())
	// log.Printf("mifare carUid: %X\n", func() []byte {

	// 	s, _ := cgo.Get("mifare_cardUid")
	// 	return s
	// }())
	// log.Printf("mifare ATR: %X\n", func() []byte {

	// 	s, _ := cgo.Get("mifare_ATQA")
	// 	return s
	// }())

	sUid, err := cgo.Get("mifare_cardUid")
	if err != nil {
		return nil, fmt.Errorf("error reading cardUid: %w", err)
	}
	fmt.Printf("mifare carUid: %X\n", sUid)
	uid := make([]byte, len(sUid))
	copy(uid, sUid)

	sSAK, err := cgo.Get("mifare_SAK")
	if err != nil {
		return nil, fmt.Errorf("error reading SAK: %w", err)
	}
	fmt.Printf("mifare SAK: %X\n", sSAK)
	sak := make([]byte, len(sSAK))
	copy(sak, sSAK)

	sATQA, err := cgo.Get("mifare_ATQA")
	if err != nil {
		return nil, fmt.Errorf("error reading ATQA: %w", err)
	}
	fmt.Printf("mifare ATQA: %X\n", sATQA)

	// sATS, err := cgo.Get("mifare_cardAts")
	// if err != nil {
	// 	return nil, fmt.Errorf("error reading ATS: %w", err)
	// }
	// fmt.Printf("mifare ATS: %X\n", sATS)

	ats := make([]byte, 0)
	if len(sak) > 0 && sak[0] == 0x20 && len(sATQA) > 1 {
		switch {
		case sATQA[0] == 0x44 && sATQA[1] == 0x03:
			ats = []byte{0x75, 0x77, 0x81, 0x02, 0x80}
		case sATQA[0] == 0x44 && sATQA[1] == 0x00:
			ats = []byte{0x75, 0x77, 0x80, 0x02, 0xC1}
		}
	}

	c := &Card{
		reader: r,
		uid:    uid,
		sak: func() byte {
			if len(sak) > 0 {
				return sak[0]
			}
			return 0xFF
		}(),
		atr:  make([]byte, 0),
		ats:  ats,
		code: 0,
	}

	return c, nil
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
