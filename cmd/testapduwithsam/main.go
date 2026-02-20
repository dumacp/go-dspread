package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dumacp/go-dspread/internal/cgo"
	"github.com/dumacp/go-dspread/pkg/cr100"
	"github.com/dumacp/smartcard/nxp/mifare"
	"github.com/dumacp/smartcard/nxp/mifare/samav3"
	"github.com/dumacp/smartcard/pcsc"
	"github.com/tarm/serial"
)

var port string
var baud int
var key string
var slot int

func init() {
	flag.StringVar(&key, "key", "", "key value to auth sam")
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

	keybytes, err := hex.DecodeString(key)
	if err != nil {
		log.Fatalf("error decoding key: %s", err)
	}

	if len(keybytes) < 16 {
		log.Fatalf("key must be 16 bytes")
	}

	if _, err := samcard.AuthHostAV2(keybytes[0:16], slot, 0, 0); err != nil {
		log.Fatalf("error AuthHostAV2 with slot \"%d\": %s", slot, err)
	}

	dev := cr100.NewDevice(serialPort)

	defer dev.Close()

	mreader := cr100.NewReaderWithDevice(dev)

	ccard, err := mreader.ConnectCard()
	if err != nil {
		log.Fatalf("error connecting card: %s", err)
	}

	defer ccard.DisconnectCard()

	atr, err = ccard.ATR()
	if err != nil {
		log.Fatalf("error reading atr: %s", err)
	} else {
		fmt.Printf("ATR: %02X, %s\n", atr, atr)
	}

	mmplus := mifare.Mplus(ccard)

	uuid, err := mmplus.UID()
	if err != nil {
		log.Fatalf("error reading uid: %s", err)
	}

	fmt.Printf("UID: %02X\n", uuid)

	// auth mplus
	ath1, err := mmplus.FirstAuthf1(0x4006)
	if err != nil {
		log.Fatalf("error auth mplus: %s", err)
	}

	fmt.Printf("auth f1 mplus: [% 02X]\n", ath1)

	divData := make([]byte, 0)
	divData = append(divData, 0x01)

	divData = append(divData, uuid...)
	for i := 0; i < len(uuid); i++ {
		divData = append(divData, uuid[len(uuid)-1-i])
	}

	fmt.Printf("divData: [% 02X]\n", divData)

	resp1, err := samcard.NonXauthMFPf1(true, 3, 10, 0, ath1, divData)
	if err != nil {
		log.Fatalf("error NonXauthMFPf1: %s", err)
	}

	fmt.Printf("response NonXauthMFPf1: [% 02X]\n", resp1)

	ath2, err := mmplus.FirstAuthf2(resp1[:len(resp1)-2])
	if err != nil {
		log.Fatalf("error auth mplus: %s", err)
	}

	fmt.Printf("auth f2 mplus: [% 02X]\n", ath2)

	resp2, err := samcard.NonXauthMFPf2(ath2)
	if err != nil {
		log.Fatalf("error NonXauthMFPf2: %s", err)
	}

	fmt.Printf("response NonXauthMFPf2: [% 02X]\n", resp2)

	dumpSession, err := samcard.DumpSessionKey()
	if err != nil {
		log.Fatalf("error DumpSessionKey: %s", err)
	}
	if dumpSession[len(dumpSession)-2] != 0x90 && dumpSession[len(dumpSession)-1] != 0x00 {
		log.Fatalf("error verify DumpSessionKey:	[% 02X]", dumpSession)
	}
	// fmt.Printf("resp3: [% X]\n", resp3)
	if err := mifare.VerifyResponseIso7816(dumpSession[:]); err != nil {
		log.Fatalf("error verify DumpSessionKey: %s", err)
	}

	keyEnc := dumpSession[0:16]
	mmplus.KeyEnc(keyEnc)
	keyMac := dumpSession[16:32]
	mmplus.KeyMac(keyMac)
	ti := dumpSession[32:36]
	mmplus.Ti(ti)
	readCounter := dumpSession[36:38]

	mmplus.ReadCounter(int(binary.LittleEndian.Uint16(readCounter)))
	writeCounter := dumpSession[38:40]
	mmplus.WriteCounter(int(binary.LittleEndian.Uint16(writeCounter)))

	rread, err := mmplus.ReadPlainMacMac(12, 5)
	if err != nil {
		log.Fatalf("error read mplus: %s", err)
	}

	fmt.Printf("read mplus: [% 02X]\n", rread)

	rread, err = mmplus.ReadPlainMacMac(16, 3)
	if err != nil {
		log.Fatalf("error read mplus: %s", err)
	}

	fmt.Printf("read mplus: [% 02X]\n", rread)

	rread, err = mmplus.ReadPlainMacMac(20, 3)
	if err != nil {
		log.Fatalf("error read mplus: %s", err)
	}

	fmt.Printf("read mplus: [% 02X]\n", rread)

	rread, err = mmplus.ReadPlainMacMac(24, 3)
	if err != nil {
		log.Fatalf("error read mplus: %s", err)
	}

	fmt.Printf("read mplus: [% 02X]\n", rread)

}
