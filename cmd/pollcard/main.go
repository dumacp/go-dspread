package main

import (
	"flag"
	"log"

	"github.com/dumacp/go-dspread/pkg/cr100"
	"github.com/tarm/serial"
)

var sport string

func init() {
	flag.StringVar(&sport, "port", "/dev/ttyACM0", "serial port")
}

func main() {

	flag.Parse()

	serialConf := serial.Config{Name: sport, Baud: 115200, ReadTimeout: 300}

	serialPort, err := serial.OpenPort(&serialConf)
	if err != nil {
		log.Fatalln(err)
	}

	defer serialPort.Close()

	// new device
	device := cr100.NewDevice(serialPort)

	// new reader
	reader := cr100.NewReaderWithDevice(device)

	// detect card, loop
	for {
		if err := func() error {
			card, err := reader.ConnectCard()
			if err != nil {
				return err
			}
			defer card.DisconnectCard()
			if atr, err := card.ATR(); err != nil {
				return err
			} else {
				log.Printf("ATR: % X\n", atr)
			}
			if uid, err := card.UID(); err != nil {
				return err
			} else {
				log.Printf("UID: % X\n", uid)
			}
			sak := card.SAK()
			log.Printf("SAK: % X\n", sak)

			if ats, err := card.ATS(); err != nil {
				return err
			} else {
				log.Printf("ATS: % X\n", ats)
			}
			return nil
		}(); err != nil {
			log.Println(err)
		}
		// time.Sleep(1 * time.Second)
	}

}
