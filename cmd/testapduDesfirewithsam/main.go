package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dumacp/go-dspread/internal/cgo"
	"github.com/dumacp/go-dspread/pkg/cr100"
	"github.com/dumacp/smartcard/nxp/mifare/desfire/ev2"
	"github.com/dumacp/smartcard/nxp/mifare/samav2"
	"github.com/dumacp/smartcard/nxp/mifare/samav3"
	"github.com/dumacp/smartcard/pcsc"
	"github.com/tarm/serial"
)

var port string
var baud int
var key string
var slot int
var keyInd int

func init() {
	flag.StringVar(&key, "keysam", "", "optional 16-byte host authentication key; empty skips SAM host authentication")
	flag.IntVar(&keyInd, "keyind", 0, "key index to auth ev2")
	flag.IntVar(&slot, "slot", 100, "slot value to auth key")
	flag.StringVar(&port, "port", "/dev/ttyACM0", "serial port")
	flag.IntVar(&baud, "baud", 115200, "baud rate")
}

func main() {
	flag.Parse()
	keybytes, err := parseHostKey(key)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("my Itoa: %s\n", cgo.MyItoA(0x16, 10))

	// Open serial port
	serialPort, err := serial.OpenPort(&serial.Config{Name: port, Baud: baud, ReadTimeout: time.Millisecond * 30})
	if err != nil {
		log.Fatalln(err)

	}

	// defer close serial port
	defer serialPort.Close()

	var reader pcsc.Reader

	ctx, err := pcsc.NewContext()

	if err != nil {
		log.Fatal(err)
	}

	readers, err := ctx.ListReaders()
	if err != nil {
		log.Fatal(err)
	}

	for i, r := range readers {
		log.Printf("reader %d: %s", i, r)
		if strings.Contains(r, fmt.Sprintf(" %s", "SAM")) {
			reader = pcsc.NewReader(ctx, r)
			break
		}
	}

	card, err := reader.ConnectCard()
	if err != nil {
		log.Fatalf("error connecting card: %s", err)
	}
	defer card.DisconnectCard()

	atr, err := card.ATR()
	if err != nil {
		log.Fatalf("error reading atr: %s", err)
	}

	fmt.Printf("ATR: %02X, %s\n", atr, atr)

	samcard := samav3.SamAV3(card)

	if respvwersion, err := samcard.GetVersion(); err != nil {
		log.Fatalf("error GetVersion: %s", err)
	} else {
		fmt.Printf("response GetVersion: [% 02X]\n", respvwersion)
	}

	if respuid, err := samcard.UID(); err != nil {
		log.Fatalf("error GetUID: %s", err)
	} else {
		fmt.Printf("response UID: [%02X]\n", respuid)
	}

	if len(keybytes) == 16 {

		if _, err := samcard.AuthHostAV2(keybytes[0:16], slot, 0, 0); err != nil {
			log.Fatalf("error AuthHostAV2 with slot \"%d\": %s", slot, err)
		}

		fmt.Printf("AuthHostAV2 with slot \"%d\" success\n", slot)
	}

	/**/

	dev := cr100.NewDevice(serialPort)

	defer dev.Close()

	mreader := cr100.NewReaderWithDevice(dev)

	ccard, err := mreader.ConnectCard()
	if err != nil {
		log.Fatalf("error connecting card: %s", err)
	}

	defer ccard.DisconnectCard()

	atrev, err := ccard.ATR()
	if err != nil {
		log.Fatalf("error reading atr: %s", err)
	} else {
		fmt.Printf("ATR: %02X\n", atrev)
	}

	d := ev2.NewDesfire(ccard)

	uid, err := d.UID()
	if err != nil {
		log.Fatalf("error get uid: %s", err)
	}

	fmt.Printf("UID: % X\n", uid)

	divData := make([]byte, 0)
	divData = append(divData, 0x01)

	divData = append(divData, uid...)
	for i := 0; i < len(uid); i++ {
		divData = append(divData, uid[len(uid)-1-i])
	}

	fmt.Printf("divData: [% 02X]\n", divData)

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

	authApp, err := d.AuthenticateEV2First(0, 3, nil)
	if err != nil {
		log.Fatalf("AuthenticateEV2First error: %s", err)
	}

	_, err = samcard.ActivateOfflineKey(keyInd, 0, divData)
	if err != nil {
		log.Fatalf("auth key %d, error ActivateOfflineKey = %s", keyInd, err)
	}
	// log.Println("ActivateOfflineKey")

	rndB, err := samcard.SAMDecipherOfflineData(samav2.AES_ALG, authApp[:])
	if err != nil {
		log.Fatalf("auth key %d, error SAMDecipherOfflineData = %s", keyInd, err)
	}
	// log.Println("SAMDecipherOfflineData")

	rndD, err := d.AuthenticateEV2FirstPart2_block_1(rndB)
	if err != nil {
		log.Fatalf("auth key %d, error AuthenticateEV2FirstPart2_block_1 = %s", keyInd, err)
	}
	// log.Println("AuthenticateEV2FirstPart2_block_1")
	rndDc, err := samcard.SAMEncipherOfflineData(samav2.AES_ALG, rndD)
	if err != nil {
		log.Fatalf("auth key %d, error SAMEncipherOfflineData = %s", keyInd, err)
	}
	// log.Println("SAMEncipherOfflineData")
	lastResponse, err := d.AuthenticateEV2FirstPart2_block_2(rndDc)
	if err != nil {
		log.Fatalf("auth key %d, error AuthenticateEV2FirstPart2_block_2 = %s", keyInd, err)
	}
	// log.Println("AuthenticateEV2FirstPart2_block_2")
	lastResponseD, err := samcard.SAMDecipherOfflineData(samav2.AES_ALG, lastResponse[:])
	if err != nil {
		log.Fatalf("auth key %d, error SAMDecipherOfflineData = %s", keyInd, err)
	}
	// log.Println("SAMDecipherOfflineData")
	sv1, sv2, err := d.AuthenticateEV2FirstPart2_block_3(lastResponseD)
	if err != nil {
		log.Fatalf("auth key %d, error AuthenticateEV2FirstPart2_block_3 = %s", keyInd, err)
	}
	// log.Println("LoadInitialVector")
	_, err = samcard.SAMLoadInitVector(samav2.AES_ALG, make([]byte, 16))
	if err != nil {
		log.Fatalf("auth key %d, error SAMLoadInitVector = %s", keyInd, err)
	}
	// log.Println("AuthenticateEV2FirstPart2_block_3")
	ksesAuthEnc, err := samcard.SAMGenerateMAC(samav2.AES_ALG, sv1)
	if err != nil {
		log.Fatalf("auth key %d, error SAMGenerateMAC = %s", keyInd, err)
	}
	// log.Println("SAMGenerateMAC")
	ksesAuthMac, err := samcard.SAMGenerateMAC(samav2.AES_ALG, sv2)
	if err != nil {
		log.Fatalf("auth key %d, error SAMGenerateMAC = %s", keyInd, err)
	}
	// log.Println("SAMGenerateMAC")
	if err := d.AuthenticateEV2FirstPart2_block_4(ksesAuthEnc, ksesAuthMac); err != nil {
		log.Fatalf("auth key %d, error AuthenticateEV2FirstPart2_block_4 = %s", keyInd, err)
	}

	log.Print("card authentication completed using the SAM offline key")

	if err := func() error {

		if data, err := d.ReadData(0x02, ev2.TargetPrimaryApp, 0x00, 0x10,
			ev2.MAC); err != nil {
			return fmt.Errorf("ReadData error: %s", err)
		} else {
			log.Printf("ReadData file 02: %X, len: %d", data, len(data))
		}

		if data, err := d.ReadData(0x04, ev2.TargetPrimaryApp, 0x00, 0x20,
			ev2.MAC); err != nil {
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
