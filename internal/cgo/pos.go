package cgo

/*
#cgo CFLAGS: -I${SRCDIR}/../../c_include
// #cgo LDFLAGS: -L${SRCDIR}/../../libs/ -ldspreadsdk
#cgo LDFLAGS: -L${SRCDIR}/../../libs/ -ldspreadsdk-for-arm-arch

#include "pos_sdk.h"
#include <stdlib.h>
#include <stdio.h>

void myitoa(int num, char* str, int base) {
    sprintf(str, "%d", num);
    // Nota: Esta implementación es básica y solo funciona con base 10.
    // Para bases diferentes, considera implementar una lógica adecuada.
}

void RSAPublicBlock(void) {

}

*/
import "C"
import (
	"fmt"
	"time"
	"unsafe"
)

func MyItoA(num int, base int) string {
	var str [32]C.char
	C.myitoa(C.int(num), &str[0], C.int(base))
	return C.GoString(&str[0])
}

func SendAPUContactless(cmd []byte, timeout time.Duration) ([]byte, error) {

	var cData *C.char
	var dataLen C.uint

	if cmd != nil {
		cData = (*C.char)(unsafe.Pointer(&cmd[0]))
		dataLen = C.uint(len(cmd))
	}

	var response [256]C.char
	// var responseLen C.int = 0

	ret := C.sendApduByNFC(cData, dataLen, C.uint(timeout.Milliseconds()), &response[0])

	if ret < 0 {
		return nil, fmt.Errorf("sendAdpuByNFC failed, ret=%d", ret)
	}

	return C.GoBytes(unsafe.Pointer(&response[0]), ret), nil
}

func PowerOnContactless(timeout time.Duration) ([]byte, error) {

	var response [256]C.char
	// var responseLen C.int = 0

	ret := C.powerOnNFC(0, C.uint(timeout.Milliseconds()), &response[0])

	if ret < 0 {
		return nil, fmt.Errorf("PowerOnContactless failed, ret=%d", ret)
	}

	return C.GoBytes(unsafe.Pointer(&response[0]), ret), nil
}

// int powerOffNFC(unsigned int timeout ,char *out );
func PowerOffContactless(timeout time.Duration) ([]byte, error) {

	var response [256]C.char
	// var responseLen C.int = 0

	ret := C.powerOffNFC(C.uint(timeout.Milliseconds()), &response[0])

	if ret < 0 {
		return nil, fmt.Errorf("PowerOffContactless failed, ret=%d", ret)
	}

	return C.GoBytes(unsafe.Pointer(&response[0]), ret), nil
}

// int doMifare(int comCode,int timeout,char* out);
func DoMifare(comCode int, timeout time.Duration) ([]byte, error) {

	var response [256]C.char
	// var responseLen C.int = 0

	ret := C.doMifare(C.int(comCode), C.int(timeout.Milliseconds()), &response[0])

	if ret < 0 {
		return nil, fmt.Errorf("DoMifare failed, ret=%d", ret)
	}

	return C.GoBytes(unsafe.Pointer(&response[0]), ret), nil
}

// int get(char* key,unsigned char *out);
func Get(key string) ([]byte, error) {
	var response [4096]C.uchar
	ret := C.get(C.CString(key), &response[0])
	if ret < 0 {
		return nil, fmt.Errorf("Get failed, ret=%d", ret)
	}
	// fmt.Printf("response: %X\n", response)
	return C.GoBytes(unsafe.Pointer(&response[0]), ret), nil
}

// int getIccTag(unsigned char encrptMode, unsigned char tagType,unsigned char tagCount, char *tagList,char* out);
func GetIccTag(encrptMode, tagType, tagCount int, tagList string) ([]byte, error) {
	var response [256]C.char
	ret := C.getIccTag(C.uchar(encrptMode), C.uchar(tagType), C.uchar(tagCount), C.CString(tagList), &response[0])
	if ret < 0 {
		return nil, fmt.Errorf("GetIccTag failed, ret=%d", ret)
	}
	return C.GoBytes(unsafe.Pointer(&response[0]), ret), nil
}

// int getCmdId(void);
func GetCmdId() (int, error) {
	ret := C.getCmdId()
	if ret < 0 {
		return 0, fmt.Errorf("GetCmdId failed, ret=%d", ret)
	}
	return int(ret), nil
}

// int on_package(unsigned char* p,int len);
func OnPackage(p []byte) (int, error) {
	ret := C.on_package((*C.uchar)(&p[0]), C.int(len(p)))
	if ret < 0 {
		return 0, fmt.Errorf("OnPackage failed, ret=%d", ret)
	}
	return int(ret), nil
}

//	enum DEVICE_CMD_RESULT{
//	    CMD_SUC=0X24,
//	    CMD_BUSY=0X23,
//	    CMD_TIMEOUT=0X25,
//	    CMD_CONTINUE=0X36,
//	    CMD_CANCEL=0X28,
//	    CMD_DECLINE=0X34
//	};
const (
	CMD_SUC      = 0x24
	CMD_BUSY     = 0x23
	CMD_TIMEOUT  = 0x25
	CMD_CONTINUE = 0x36
	CMD_CANCEL   = 0x28
	CMD_DECLINE  = 0x34
)

// int get_response_result();
func GetResponseResult() (int, error) {
	ret := C.get_response_result()
	if ret < 0 {
		return 0, fmt.Errorf("GetResponseResult failed, ret=%d", ret)
	}
	return int(ret), nil
}

/*
// int packSwipeAndIC(int tradeMode,int tradeType,int MSRDebitCreditMode,char*TradeTime,int timeout, char *out);
func PackSwipeAndIC(tradeMode, tradeType, MSRDebitCreditMode int, TradeTime string, timeout time.Duration) ([]byte, error) {
	var response [256]C.char
	ret := C.packSwipeAndIC(C.int(tradeMode), C.int(tradeType), C.int(MSRDebitCreditMode), C.CString(TradeTime), C.int(timeout.Milliseconds()), &response[0])
	if ret < 0 {
		return nil, fmt.Errorf("PackSwipeAndIC failed, ret=%d", ret)
	}
	return C.GoBytes(unsafe.Pointer(&response[0]), ret), nil
}
/*/
/**/
// int packQueryLatestCmdResult(char *out);
func PackQueryLatestCmdResult() ([]byte, error) {
	var response [256]C.char
	ret := C.packQueryLatestCmdResult(&response[0])
	if ret < 0 {
		return nil, fmt.Errorf("PackQueryLatestCmdResult failed, ret=%d", ret)
	}
	return C.GoBytes(unsafe.Pointer(&response[0]), ret), nil
}

/**/
