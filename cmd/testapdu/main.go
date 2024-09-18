package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/dumacp/go-dspread/internal/cgo"
	"github.com/tarm/serial"
)

var port string
var baud int

func init() {
	flag.StringVar(&port, "port", "/dev/ttyACM0", "serial port")
	flag.IntVar(&baud, "baud", 115200, "baud rate")
}

func main() {
	flag.Parse()

	fmt.Printf("my Itoa: %s\n", cgo.MyItoA(0x16, 10))

	// Open serial port
	serialPort, err := serial.OpenPort(&serial.Config{Name: port, Baud: baud, ReadTimeout: time.Millisecond * 30})
	if err != nil {
		log.Fatalln(err)

	}

	// defer close serial port
	defer serialPort.Close()

	cmdid := func() int {
		c, err := cgo.GetCmdId()
		if err != nil {
			log.Printf("error reading from nfc GetCmdId: %v", err)
			return -1
		}
		fmt.Printf("function cmdid: 0x%X\n", c)
		return c
	}

	// poff, err := cgo.PowerOffContactless(time.Millisecond * 1000)
	// if err != nil {
	// 	log.Fatalln(err)
	// }

	// log.Printf("apdu to send: %X\n", poff)

	// if _, err := serialPort.Write(poff); err != nil {
	// 	log.Fatalf("error writing to serial port: %v", err)
	// }

	// n0, err := serialPort.Read(buff)
	// if err != nil {
	// 	log.Fatalf("error reading from serial port: %v", err)
	// }

	// log.Printf("Response poff: %X, %q\n", buff[0:n0], buff[0:n0])

	// // Power on contactless
	// fmt.Println("power on")
	// t0 := time.Now()
	// pon, err := cgo.PowerOnContactless(time.Millisecond * 10)
	// if err != nil {
	// 	log.Println(err)
	// 	return
	// }

	// if _, err := Transmit(serialPort, pon); err != nil {
	// 	log.Println(err)
	// 	return
	// }

	// //query cmdid
	// for {
	// 	if c := cmdid(); c == cgo.CMD_BUSY || c == cgo.CMD_CONTINUE {
	// 		fmt.Printf("while cmdid: 0x%X\n", c)
	// 		if out, err := cgo.PackQueryLatestCmdResult(); err != nil {
	// 			log.Printf("error packing query cmdid: %v", err)
	// 			return
	// 		} else {
	// 			Transmit(serialPort, out)
	// 		}
	// 	} else {
	// 		break
	// 	}
	// }

	// if c := cmdid(); c == 0 || cgo.CMD_CANCEL == cgo.CMD_TIMEOUT {
	// 	log.Printf("cmdid: 0x%X\n", c)
	// 	return
	// } else {

	// 	log.Printf("prog 0 cmdid: 0x%X\n", c)
	// }

	// fmt.Printf("///////////////  time poll: %v\n", time.Since(t0).Milliseconds())
	// data0, err := cgo.Get("Atr")
	// if err != nil {
	// 	log.Printf("error reading from nfc: %v", err)
	// 	return
	// }

	// log.Printf("Data NFC Atr: %X\n", data0)

	// data1, err := cgo.Get("HasCard")
	// if err != nil {
	// 	log.Printf("error reading from nfc: %v", err)
	// 	return
	// }

	// log.Printf("Data NFC HasCard: %X\n", data1)

	/**/

	t0 := time.Now()
	poll, err := cgo.DoMifare(0x01, time.Millisecond*10)
	if err != nil {
		log.Println(err)
		return
	}

	if _, err := Transmit(serialPort, poll); err != nil {
		log.Println(err)
		return
	}
	fmt.Printf("///////////////  time poll: %v\n", time.Since(t0).Milliseconds())

	log.Printf("mifare SAK: %X\n", func() []byte {

		s, _ := cgo.Get("mifare_SAK")
		return s
	}())
	log.Printf("mifare carUid: %X\n", func() []byte {

		s, _ := cgo.Get("mifare_cardUid")
		return s
	}())
	log.Printf("mifare ATR: %X\n", func() []byte {

		s, _ := cgo.Get("mifare_ATQA")
		return s
	}())

	// Send APDU command
	apdu := []byte{0x70, 0x00, 0x40, 0x00}
	// apdu := []byte{0x90, 0x51, 0x00, 0x00, 0x00}
	// apdu := []byte{0x00, 0x84, 0x00, 0x00, 0x08}

	t1 := time.Now()
	fmt.Printf("sen apdu")
	cmd, err := cgo.SendAPUContactless(apdu, time.Millisecond*20)
	if err != nil {
		log.Println(err)
		return
	}

	Transmit(serialPort, cmd)

	for {
		if c := cmdid(); c == cgo.CMD_BUSY || c == cgo.CMD_CONTINUE {
			fmt.Printf("while cmdid: 0x%X\n", c)
			if out, err := cgo.PackQueryLatestCmdResult(); err != nil {
				log.Printf("error packing query cmdid: %v", err)
				return
			} else {
				Transmit(serialPort, out)
			}
		} else {
			break

		}
	}

	if c := cmdid(); c == 0 || c == cgo.CMD_CANCEL || c == cgo.CMD_TIMEOUT {
		log.Printf("cmdid: 0x%X\n", c)
		return
	} else {
		log.Printf("prog 1 cmdid: 0x%X\n", c)
	}

	fmt.Printf("/////////////// time first auth: %v\n", time.Since(t1).Milliseconds())
	dataLen, err := cgo.Get("ApduLen")
	if err != nil {
		log.Printf("error reading from nfc: %v", err)
		return
	}

	log.Printf("Data Len: %X\n", dataLen)

	dataApduResult, err := cgo.Get("ApduResult")
	if err != nil {
		log.Printf("error reading from nfc: %v", err)
		return
	}

	log.Printf("Data dataApduResult: %X\n", dataApduResult)

	dataApduEncrypt, err := cgo.Get("ApduEncrpt")
	if err != nil {
		log.Printf("error reading from nfc: %v", err)
		return
	}

	log.Printf("Data dataApduEncrypt: %X\n", dataApduEncrypt)

}
