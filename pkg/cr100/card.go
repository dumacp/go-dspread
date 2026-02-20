package cr100

import (
	"fmt"
	"log"
	"time"

	"github.com/dumacp/go-dspread/internal/cgo"
	smartcard "github.com/dumacp/smartcard"
)

type Card struct {
	smartcard.ICard
	code   int
	sak    byte
	atr    []byte
	uid    []byte
	ats    []byte
	reader *Reader
}

// Apdu send apdu to card
func (c *Card) Apdu(apdu []byte) ([]byte, error) {

	fmt.Printf("APDU: %02X\n", apdu)

	timeout := 300 * time.Millisecond

	cmd, err := cgo.SendAPUContactless(apdu, timeout)
	if err != nil {
		return nil, fmt.Errorf("error sending apdu: %w", err)
	}

	// ctx, cancel := context.WithTimeout(context.Background(), timeout)
	// defer cancel()

	t1 := time.Now()

	if _, err := c.reader.dev.Transmit(cmd, 300*time.Millisecond); err != nil {
		return nil, fmt.Errorf("error transmitting: %w", err)
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

	for {
		if cod := cmdid(); cod == cgo.CMD_BUSY || cod == cgo.CMD_CONTINUE {
			fmt.Printf("while cmdid: 0x%X\n", c)
			if out, err := cgo.PackQueryLatestCmdResult(); err != nil {
				return nil, fmt.Errorf("error packing query cmdid: %w", err)
			} else {
				fmt.Printf("out: [% 02X]\n", out)
				c.reader.dev.Transmit(out, 3*time.Millisecond)
			}
		} else {
			break
		}
	}

	if c := cmdid(); c == 0 || c == cgo.CMD_CANCEL || c == cgo.CMD_TIMEOUT {
		// log.Printf("cmdid: 0x%X\n", c)
		// return
		return nil, fmt.Errorf("error cmdid: 0x%X", c)
	} else {
		log.Printf("prog 1 cmdid: 0x%X\n", c)
	}

	fmt.Printf("/////////////// time apdu: %v\n", time.Since(t1).Milliseconds())
	dataLen, err := cgo.Get("ApduLen")
	if err != nil {
		// log.Printf("error reading from nfc: %v", err)
		// return
		return nil, fmt.Errorf("error reading from nfc: %w", err)
	}

	log.Printf("Data Len: %X (%d)\n", dataLen, len(dataLen))

	dataApduResult, err := cgo.Get("ApduResult")
	if err != nil {
		// log.Printf("error reading from nfc: %v", err)
		// return
		return nil, fmt.Errorf("error reading from nfc: %w", err)
	}

	log.Printf("Data dataApduResult: %X\n", dataApduResult)

	dataApduEncrypt, err := cgo.Get("ApduEncrpt")
	if err != nil {
		// log.Printf("error reading from nfc: %v", err)
		// return
		return nil, fmt.Errorf("error reading from nfc: %w", err)
	}

	log.Printf("Data dataApduEncrypt: %X\n", dataApduEncrypt)

	return dataApduEncrypt, nil

}

func (c *Card) ATR() ([]byte, error) {
	return c.atr, nil
}

func (c *Card) GetData(_ byte) ([]byte, error) {
	return make([]byte, 0), nil
}

func (c *Card) UID() ([]byte, error) {
	return c.uid, nil
}

func (c *Card) ATS() ([]byte, error) {
	return c.ats, nil
}

func (c *Card) SAK() byte {
	return c.sak
}

func (c *Card) DisconnectCard() error {
	fmt.Printf("disconnect card\n")
	time.Sleep(time.Millisecond * 1000)
	// poll, err := cgo.DoMifare(0x0e, time.Millisecond*10)
	// if err != nil {
	// 	return fmt.Errorf("error doing mifare comand 0x0E: %w", err)
	// }

	// // ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	// // defer cancel()
	// if _, err := c.reader.dev.Transmit(poll, 300*time.Millisecond); err != nil {
	// 	return fmt.Errorf("error transmiting end transaction: %w", err)
	// }

	return nil
}

func (c *Card) DisconnectResetCard() error {
	return c.DisconnectCard()
}

func (c *Card) DisconnectUnpowerCard() error {
	return c.DisconnectCard()
}

func (c *Card) DisconnectEjectCard() error {
	return c.DisconnectCard()
}

func (c *Card) EndTransactionResetCard() error {
	return c.DisconnectCard()
}
