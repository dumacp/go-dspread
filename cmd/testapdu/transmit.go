package main

import (
	"io"
	"log"
	"time"

	"fmt"

	"github.com/dumacp/go-dspread/internal/cgo"
)

func Transmit(s io.ReadWriter, in []byte) ([]byte, error) {

	res := make([]byte, 0)
	buff := make([]byte, 256)
	log.Printf("apdu to send: 0x%X\n", in)
	if _, err := s.Write(in); err != nil {
		return nil, fmt.Errorf("error writing to serial port: %v", err)
	}

	for range make([]int, 10) {
		n3, err := s.Read(buff)
		if err != nil {
			fmt.Printf("error reading from serial port: %v\n", err)
			continue
		}
		res = append(res, buff[0:n3]...)
		// for {
		// 	n3, err := s.Read(buff)
		// 	if err != nil {
		// 		break
		// 	}
		// 	res = append(res, buff[0:n3]...)
		// }

		log.Printf("Response transmit: %X\n", res)

		t0 := time.Now()

		if _, err := cgo.OnPackage(res); err != nil {
			return nil, fmt.Errorf("error on package: %v", err)
		}
		fmt.Printf("time onpackage: %v\n", time.Since(t0).Milliseconds())
		break
	}

	return res, nil

}
