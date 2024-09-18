package nfc

import "github.com/dumacp/go-dspread/internal/cgo"

func GetAllDataNFC() ([]byte, error) {

	return cgo.GetIccTag(0, 1, 0, "")

}
