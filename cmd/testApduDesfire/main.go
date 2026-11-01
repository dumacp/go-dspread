package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/dumacp/go-dspread/internal/cgo"
	"github.com/dumacp/go-dspread/pkg/cr100"
	"github.com/dumacp/smartcard"
	"github.com/dumacp/smartcard/nxp/mifare/desfire/ev2"
	"github.com/tarm/serial"
)

var port string
var baud int
var key string
var slot int
var readLen04 int
var apduCommandTimeout uint
var apduSerialTimeout time.Duration
var powerOnNFC bool

// ---------- LOGGING: CR100 frame capture over serial port ----------

type serialLogger struct {
	io.ReadWriteCloser
}

func (w *serialLogger) Write(p []byte) (int, error) {
	fmt.Printf("\n████████████████████████████████████████████████████████\n")
	fmt.Printf("  CR100 FRAME (sent to reader via RS-232):\n")
	fmt.Printf("  Hex: % X\n", p)
	fmt.Printf("  Length: %d bytes\n", len(p))
	fmt.Printf("████████████████████████████████████████████████████████\n")
	return w.ReadWriteCloser.Write(p)
}

// ---------- LOGGING: Native Desfire APDU annotation ----------

type loggingCard struct {
	smartcard.ICard
}

func (c *loggingCard) Apdu(apdu []byte) ([]byte, error) {
	reportAPDU(apdu)
	return c.ICard.Apdu(apdu)
}

func reportAPDU(apdu []byte) {
	if len(apdu) == 0 {
		return
	}
	ins := apdu[0]

	fmt.Printf("\n═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  📋 NATIVE DESFIRE EV2 APDU\n")
	fmt.Printf("  Full hex: [ % X ]\n", apdu)
	fmt.Printf("  Total bytes: %d\n", len(apdu))
	fmt.Printf("───────────────────────────────────────────────────────────────\n")

	switch ins {
	case 0x5A:
		fmt.Printf("  ▶ Command: SelectApplication (0x5A)\n")
		fmt.Printf("  ───────────────────────────────────────────────────────\n")
		fmt.Printf("    %-8s | %-12s | %s\n", "Offset", "Bytes", "Description")
		fmt.Printf("    %-8s-|-%12s-|-%s\n", "────────", "────────────", "────────────────────────────────────")
		fmt.Printf("    0       | [5A]        | INS = SelectApplication\n")
		if len(apdu) >= 4 {
			aid := apdu[1:4]
			aidVal := uint32(aid[2])<<16 | uint32(aid[1])<<8 | uint32(aid[0])
			fmt.Printf("    1-3     | [%02X %02X %02X]   | AID (Application ID) = 0x%06X\n", aid[0], aid[1], aid[2], aidVal)
		}
		if len(apdu) >= 7 {
			aid2 := apdu[4:7]
			aid2Val := uint32(aid2[2])<<16 | uint32(aid2[1])<<8 | uint32(aid2[0])
			fmt.Printf("    4-6     | [%02X %02X %02X]   | AID2 (optional secondary app) = 0x%06X\n", aid2[0], aid2[1], aid2[2], aid2Val)
		}
		fmt.Printf("\n  ※ PLAIN mode (no MAC, no encryption)\n")

	case 0x71:
		fmt.Printf("  ▶ Command: AuthenticateEV2First (0x71)\n")
		fmt.Printf("  ───────────────────────────────────────────────────────\n")
		fmt.Printf("    %-8s | %-12s | %s\n", "Offset", "Bytes", "Descripción")
		fmt.Printf("    %-8s-|-%12s-|-%s\n", "────────", "────────────", "────────────────────────────────────")
		fmt.Printf("    0       | [71]        | INS = AuthenticateEV2First\n")
		if len(apdu) >= 2 {
			keyNo := apdu[1] & 0x7F
			secFlag := (apdu[1] >> 7) & 1
			appStr := "Primary App"
			if secFlag == 1 {
				appStr = "Secondary App"
			}
			fmt.Printf("    1       | [%02X]        | KeyNo (key number) = %d (bits 0-6), %s (bit7=%d)\n", apdu[1], keyNo, appStr, secFlag)
		}
		if len(apdu) >= 3 {
			fmt.Printf("    2       | [%02X]        | PCD Capabilities Length = %d\n", apdu[2], apdu[2])
		}
		if len(apdu) > 3 {
			capLen := int(apdu[2])
			if capLen > 0 && 3+capLen <= len(apdu) {
				fmt.Printf("    3..%d   | [% X] | PCD Capabilities (%d bytes)\n", 2+capLen, apdu[3:3+capLen], capLen)
			}
		}
		if len(apdu) >= 8 {
			mac := apdu[len(apdu)-8:]
			fmt.Printf("    %d..%d   | [% X] | CMAC (8 bytes, truncated AES-CMAC)\n", len(apdu)-8, len(apdu)-1, mac)
		}

	case 0xAF:
		fmt.Printf("  ▶ Command: AuthenticateEV2FirstPart2 (0xAF)\n")
		fmt.Printf("  ───────────────────────────────────────────────────────\n")
		fmt.Printf("    %-8s | %-12s | %s\n", "Offset", "Bytes", "Descripción")
		fmt.Printf("    %-8s-|-%12s-|-%s\n", "────────", "────────────", "────────────────────────────────────")
		fmt.Printf("    0       | [AF]        | INS = AuthenticateEV2First Part2\n")
		if len(apdu) >= 33 {
			fmt.Printf("    1-32    | [% X] | rndD encrypted (AES-CBC with IV=0)\n", apdu[1:33])
			fmt.Printf("             |             |   ├─ [01..16] rndA (PCD random, 16 bytes)\n")
			fmt.Printf("             |             |   └─ [17..32] rndB rotated (rndB <<< 1 byte)\n")
		}
		if len(apdu) >= 41 {
			mac := apdu[33:41]
			fmt.Printf("    33-40   | [% X] | CMAC (8 bytes)\n", mac)
		}

	case 0xAD:
		fmt.Printf("  ▶ Command: ReadData (0xAD)\n")
		fmt.Printf("  ───────────────────────────────────────────────────────\n")
		fmt.Printf("    %-8s | %-12s | %s\n", "Offset", "Bytes", "Descripción")
		fmt.Printf("    %-8s-|-%12s-|-%s\n", "────────", "────────────", "────────────────────────────────────")
		fmt.Printf("    0       | [AD]        | INS = ReadData\n")
		if len(apdu) >= 8 {
			fileNo := apdu[1] & 0x7F
			secFlag := (apdu[1] >> 7) & 1
			appStr := "Primary App"
			if secFlag == 1 {
				appStr = "Secondary App"
			}
			offset := uint32(apdu[2]) | uint32(apdu[3])<<8 | uint32(apdu[4])<<16
			length := uint32(apdu[5]) | uint32(apdu[6])<<8 | uint32(apdu[7])<<16
			fmt.Printf("    1       | [%02X]        | FileNo = %d (bits 0-6), %s (bit7=%d)\n", apdu[1], fileNo, appStr, secFlag)
			fmt.Printf("    2-4     | [%02X %02X %02X]   | Offset = %d (0x%06X, Little-Endian 3 bytes)\n", apdu[2], apdu[3], apdu[4], offset, offset)
			fmt.Printf("    5-7     | [%02X %02X %02X]   | Length = %d (0x%06X, Little-Endian 3 bytes)\n", apdu[5], apdu[6], apdu[7], length, length)
		}
		if len(apdu) > 8 {
			mac := apdu[8:]
			fmt.Printf("    8..%d   | [% X] | CMAC (%d bytes)\n", 7+len(mac), mac, len(mac))
		}

	case 0x6C:
		fmt.Printf("  ▶ Command: GetValue (0x6C)\n")
		fmt.Printf("  ───────────────────────────────────────────────────────\n")
		fmt.Printf("    %-8s | %-12s | %s\n", "Offset", "Bytes", "Descripción")
		fmt.Printf("    %-8s-|-%12s-|-%s\n", "────────", "────────────", "────────────────────────────────────")
		fmt.Printf("    0       | [6C]        | INS = GetValue\n")
		if len(apdu) >= 2 {
			fileNo := apdu[1] & 0x7F
			secFlag := (apdu[1] >> 7) & 1
			appStr := "Primary App"
			if secFlag == 1 {
				appStr = "Secondary App"
			}
			fmt.Printf("    1       | [%02X]        | FileNo = %d (bits 0-6), %s (bit7=%d)\n", apdu[1], fileNo, appStr, secFlag)
		}
		if len(apdu) > 2 {
			mac := apdu[2:]
			fmt.Printf("    2..%d   | [% X] | CMAC (%d bytes)\n", 1+len(mac), mac, len(mac))
		}

	case 0xAB:
		fmt.Printf("  ▶ Command: ReadRecords (0xAB)\n")
		fmt.Printf("  ───────────────────────────────────────────────────────\n")
		fmt.Printf("    %-8s | %-12s | %s\n", "Offset", "Bytes", "Descripción")
		fmt.Printf("    %-8s-|-%12s-|-%s\n", "────────", "────────────", "────────────────────────────────────")
		fmt.Printf("    0       | [AB]        | INS = ReadRecords\n")
		if len(apdu) >= 8 {
			fileNo := apdu[1] & 0x7F
			secFlag := (apdu[1] >> 7) & 1
			appStr := "Primary App"
			if secFlag == 1 {
				appStr = "Secondary App"
			}
			recNo := uint32(apdu[2]) | uint32(apdu[3])<<8 | uint32(apdu[4])<<16
			recCount := uint32(apdu[5]) | uint32(apdu[6])<<8 | uint32(apdu[7])<<16
			fmt.Printf("    1       | [%02X]        | FileNo = %d (bits 0-6), %s (bit7=%d)\n", apdu[1], fileNo, appStr, secFlag)
			fmt.Printf("    2-4     | [%02X %02X %02X]   | RecNo (start record) = %d (Little-Endian 3 bytes)\n", apdu[2], apdu[3], apdu[4], recNo)
			fmt.Printf("    5-7     | [%02X %02X %02X]   | RecCount = %d (Little-Endian 3 bytes)\n", apdu[5], apdu[6], apdu[7], recCount)
		}
		if len(apdu) > 8 {
			mac := apdu[8:]
			fmt.Printf("    8..%d   | [% X] | CMAC (%d bytes)\n", 7+len(mac), mac, len(mac))
		}

	case 0x6F:
		fmt.Printf("  ▶ Command: GetFileIDs (0x6F)\n")
		fmt.Printf("  ───────────────────────────────────────────────────────\n")
		fmt.Printf("    %-8s | %-12s | %s\n", "Offset", "Bytes", "Descripción")
		fmt.Printf("    %-8s-|-%12s-|-%s\n", "────────", "────────────", "────────────────────────────────────")
		fmt.Printf("    0       | [6F]        | INS = GetFileIDs\n")
		if len(apdu) > 1 {
			mac := apdu[1:]
			fmt.Printf("    1..%d   | [% X] | CMAC (%d bytes)\n", len(mac), mac, len(mac))
		}

	default:
		fmt.Printf("  ▶ Unknown command (INS = 0x%02X)\n", ins)
		fmt.Printf("  ───────────────────────────────────────────────────────\n")
		fmt.Printf("    Hex completo: [ % X ]\n", apdu)
	}

	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  ↑ Native APDU → C library sendApduByNFC()\n")
	fmt.Printf("  ↓ C library wraps it in CR100 protocol and sends via RS-232:\n\n")
}

func init() {
	flag.StringVar(&key, "key", "00000000000000000000000000000000", "16-byte DESFire card key in hexadecimal")
	flag.IntVar(&slot, "slot", 100, "DESFire application key number")
	flag.IntVar(&readLen04, "readlen04", 0x40, "length to read from file 04")
	flag.UintVar(&apduCommandTimeout, "apdu-timeout", 300, "raw timeout value passed to sendApduByNFC")
	flag.DurationVar(&apduSerialTimeout, "apdu-serial-timeout", 300*time.Millisecond, "serial response timeout for APDU commands")
	flag.BoolVar(&powerOnNFC, "poweron-nfc", false, "connect using vendor powerOnNFC flow instead of doMifare poll")
	flag.StringVar(&port, "port", "/dev/ttyACM0", "serial port")
	flag.IntVar(&baud, "baud", 115200, "baud rate")
}

func main() {
	flag.Parse()
	cr100.APDUCommandTimeoutValue = apduCommandTimeout
	cr100.APDUSerialTimeout = apduSerialTimeout
	cr100.UsePowerOnNFCConnect = powerOnNFC

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

	// Envolver el puerto serie para capturar las tramas CR100
	logSerial := &serialLogger{ReadWriteCloser: serialPort}
	dev := cr100.NewDevice(logSerial)

	defer dev.Close()

	mreader := cr100.NewReaderWithDevice(dev)

	ccard, err := mreader.ConnectCard()
	if err != nil {
		log.Fatalf("error connecting card: %s", err)
	}

	defer ccard.DisconnectCard()

	// Envolver la tarjeta para anotar los APDUs nativos Desfire
	ccard = &loggingCard{ICard: ccard}

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

	/**
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
	/**/

	authApp, err := d.AuthenticateEV2First(0, slot, nil)
	if err != nil {
		log.Fatalf("AuthenticateEV2First error: %s", err)
	}

	authApp2, err := d.AuthenticateEV2FirstPart2(keybytes[0:16], authApp)
	if err != nil {
		log.Fatalf("AuthenticateEV2FirstPart2 error: %s", err)
	}

	log.Printf("**** auth APP AES sucess: [% X]", authApp2)

	if err := func() error {

		if data, err := d.ReadData(0x02, ev2.TargetPrimaryApp, 0x00, 0x10,
			ev2.FULL); err != nil {
			return fmt.Errorf("ReadData error: %s", err)
		} else {
			log.Printf("ReadData file 02: %X, len: %d", data, len(data))
		}

		if data, err := d.ReadData(0x04, ev2.TargetPrimaryApp, 0x00, readLen04,
			ev2.FULL); err != nil {
			return fmt.Errorf("ReadData error: %s", err)
		} else {
			log.Printf("ReadData file 04: %X, len: %d", data, len(data))
		}

		//if data, err := d.GetValue(0x05, ev2.TargetPrimaryApp,
		//	ev2.FULL); err != nil {
		//	return fmt.Errorf("GetValue error: %s", err)
		//} else {

		//	dataInt := binary.LittleEndian.Uint32(data[:4])
		//	log.Printf("GetValue: %d, %X, len: %d", dataInt, data, len(data))
		//}

		//if data, err := d.ReadRecords(0x07, ev2.TargetPrimaryApp, 0x00, 0x01, 32,
		//	ev2.FULL); err != nil {
		//	return fmt.Errorf("ReadRecords error: %s", err)
		//} else {
		//	log.Printf("ReadRecords 06: %X, len: %d", data, len(data))
		//}
		//if data, err := d.ReadRecords(0x06, ev2.TargetPrimaryApp, 0x00, 0x01, 32,
		//	ev2.FULL); err != nil {
		//	return fmt.Errorf("ReadRecords error: %s", err)
		//} else {
		//	log.Printf("ReadRecords 07: %X, len: %d", data, len(data))
		//}
		return nil
	}(); err != nil {
		log.Fatalf("error reading data: %s", err)
	}

}
