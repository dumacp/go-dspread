package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/dumacp/go-dspread/internal/cgo"
	"github.com/dumacp/go-dspread/pkg/cr100"
	"github.com/dumacp/smartCard/nxp/mifare/desfire/ev2"
	"github.com/tarm/serial"
)

var port string
var baud int
var key string
var slot int

func init() {
	flag.StringVar(&key, "key", "00000000000000000000000000000000", "key value to auth sam")
	flag.IntVar(&slot, "slot", 100, "slot value to auth key")
	flag.StringVar(&port, "port", "/dev/ttyACM0", "serial port")
	flag.IntVar(&baud, "baud", 115200, "baud rate")
}

func main() {
	flag.Parse()

	fmt.Printf("my Itoa: %s\n", cgo.MyItoA(0x16, 10))

	// Open serial port
	serialPort, err := serial.OpenPort(&serial.Config{Name: port, Baud: baud, ReadTimeout: time.Millisecond * 30})
	if err != nil {
		log.Fatalf("error open serial port: %s", err)

	}

	// defer close serial port
	defer serialPort.Close()

	keybytes, err := hex.DecodeString(key)
	if err != nil {
		log.Fatalf("error decoding key: %s", err)
	}

	if len(keybytes) < 16 {
		log.Fatalf("key must be 16 bytes")
	}

	dev := cr100.NewDevice(serialPort)

	defer dev.Close()

	mreader := cr100.NewReaderWithDevice(dev)

	ccard, err := mreader.ConnectCard()
	if err != nil {
		log.Fatalf("error connecting card: %s", err)
	}

	defer ccard.DisconnectCard()

	atr, err := ccard.ATR()
	if err != nil {
		log.Fatalf("error reading atr: %s", err)
	} else {
		fmt.Printf("ATR: %02X, %s\n", atr, atr)
	}

	d := ev2.NewDesfire(ccard)

	uid, err := d.UID()
	if err != nil {
		log.Fatalf("error get uid: %s", err)
	}

	fmt.Printf("UID: % X\n", uid)

	aid := []byte{0x01, 0x00, 0x00}

	// // se selecciona la app 0x0000001 (antes estab seleccionado todo el PICC)
	// if err := d.SelectApplication(aid, nil); err != nil {
	// 	return fmt.Errorf("SelectApplication error: %s", err)
	// }

	// se selecciona la app 0x0000001 (antes estab seleccionado todo el PICC)
	if err := d.SelectApplication(aid, nil); err != nil {
		log.Fatalf("SelectApplication error: %s", err)
	}

	/**/
	// auth con la app seleccionada
	authApp, err := d.AuthenticateEV2First(0, 0, nil)
	if err != nil {
		log.Fatalf("AuthenticateEV2First error: %s", err)
	}

	authApp2, err := d.AuthenticateEV2FirstPart2(keybytes[0:16], authApp)
	if err != nil {
		log.Fatalf("AuthenticateEV2FirstPart2 error: %s", err)
	}

	log.Printf("**** auth APP AES sucess: [% X]", authApp2)

	// se listas los archivos creados en la tarjeta
	fileIDs, err := d.GetFileIDs()
	if err != nil {
		log.Fatalf("GetFileIDs error: %s", err)
	}

	log.Printf("file IDs response: [% X]", fileIDs)

	authApp, err = d.AuthenticateEV2First(0, 4, nil)
	if err != nil {
		log.Fatalf("AuthenticateEV2First error: %s", err)
	}

	authApp2, err = d.AuthenticateEV2FirstPart2(keybytes[0:16], authApp)
	if err != nil {
		log.Fatalf("AuthenticateEV2FirstPart2 error: %s", err)
	}

	log.Printf("**** auth APP AES sucess: [% X]", authApp2)

	if err := func() error {

		if data, err := d.ReadData(0x02, ev2.TargetPrimaryApp, 0x00, 0x10,
			ev2.MAC); err != nil {
			return fmt.Errorf("ReadData error: %s", err)
		} else {
			log.Printf("ReadData file 02: %X, len: %d", data, len(data))
		}

		if data, err := d.ReadData(0x03, ev2.TargetPrimaryApp, 0x00, 0x20,
			ev2.MAC); err != nil {
			return fmt.Errorf("ReadData error: %s", err)
		} else {
			log.Printf("ReadData file 03: %X, len: %d", data, len(data))
		}

		if data, err := d.GetValue(0x05, ev2.TargetPrimaryApp,
			ev2.FULL); err != nil {
			return fmt.Errorf("GetValue error: %s", err)
		} else {

			dataInt := binary.LittleEndian.Uint32(data[:4])
			log.Printf("GetValue: %d, %X, len: %d", dataInt, data, len(data))
		}

		if data, err := d.ReadRecords(0x07, ev2.TargetPrimaryApp, 0x00, 0x01, 32,
			ev2.FULL); err != nil {
			return fmt.Errorf("ReadRecords error: %s", err)
		} else {
			log.Printf("ReadRecords 06: %X, len: %d", data, len(data))
		}
		if data, err := d.ReadRecords(0x06, ev2.TargetPrimaryApp, 0x00, 0x01, 32,
			ev2.FULL); err != nil {
			return fmt.Errorf("ReadRecords error: %s", err)
		} else {
			log.Printf("ReadRecords 07: %X, len: %d", data, len(data))
		}
		return nil
	}(); err != nil {
		log.Fatalf("error reading data: %s", err)
	}

}
