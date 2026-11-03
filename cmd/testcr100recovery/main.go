// Standalone, read-only CR100 recovery experiment. No production packages change.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	sdk "github.com/dumacp/go-dspread/internal/cgo"
	"github.com/dumacp/go-dspread/pkg/cr100"
	"github.com/dumacp/smartcard"
	"github.com/dumacp/smartcard/nxp/mifare/desfire/ev2"
	"github.com/dumacp/smartcard/nxp/mifare/samav2"
	"github.com/dumacp/smartcard/nxp/mifare/samav3"
	"github.com/dumacp/smartcard/pcsc"
	"github.com/tarm/serial"
)

type config struct {
	port, samName, recovery string
	baud, samEntry, cardKey int
	duration, pause, settle time.Duration
	list, selectOnly        bool
	resetBefore             string
	bounded                 bool
}

// Reject every command except application select, EV2 authentication and ReadData.
// In particular WriteData, WriteRecord and CommitTransaction cannot reach hardware.
type readOnlyCard struct{ smartcard.ICard }

func (c readOnlyCard) Apdu(apdu []byte) ([]byte, error) {
	if len(apdu) == 0 {
		return nil, errors.New("empty command")
	}
	switch apdu[0] {
	case 0x5a, 0x71, 0xaf, 0xad:
	default:
		return nil, fmt.Errorf("read-only probe rejects command %02X", apdu[0])
	}
	start := time.Now()
	response, err := c.ICard.Apdu(apdu)
	log.Printf("PROBE_APDU cmd=%02X tx=%X rx=%X elapsed_ms=%d err=%v", apdu[0], apdu, response, time.Since(start).Milliseconds(), err)
	if err == nil && len(response) == 0 {
		return nil, errors.New("CR100 returned empty APDU payload")
	}
	return response, err
}

type result struct {
	Time      string `json:"time"`
	Attempt   int    `json:"attempt"`
	Recovery  string `json:"recovery"`
	UID       string `json:"uid,omitempty"`
	Stage     string `json:"stage"`
	OK        bool   `json:"ok"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Error     string `json:"error,omitempty"`
}

func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func authenticate(d *ev2.Desfire, sam *samav3.CsamAv3, uid []byte, keyNo, entry int, stage *string) error {
	*stage = "auth1_card_challenge"
	challenge, err := d.AuthenticateEV2First(ev2.TargetPrimaryApp, keyNo, nil)
	if err != nil {
		return err
	}
	div := append([]byte{1}, uid...)
	for i := len(uid) - 1; i >= 0; i-- {
		div = append(div, uid[i])
	}
	*stage = "sam_activate_key"
	if _, err = sam.ActivateOfflineKey(entry, 0, div); err != nil {
		return err
	}
	*stage = "sam_decrypt_challenge"
	rndB, err := sam.SAMDecipherOfflineData(samav2.AES_ALG, challenge)
	if err != nil {
		return err
	}
	if len(rndB) != 16 {
		return fmt.Errorf("unexpected RndB length %d", len(rndB))
	}
	data, err := d.AuthenticateEV2FirstPart2_block_1(rndB)
	if err != nil {
		return err
	}
	*stage = "sam_encrypt_auth2"
	encrypted, err := sam.SAMEncipherOfflineData(samav2.AES_ALG, data)
	if err != nil {
		return err
	}
	*stage = "auth2_card_response"
	response, err := d.AuthenticateEV2FirstPart2_block_2(encrypted)
	if err != nil {
		return err
	}
	*stage = "sam_decrypt_auth2"
	plain, err := sam.SAMDecipherOfflineData(samav2.AES_ALG, response)
	if err != nil {
		return err
	}
	if len(plain) != 32 {
		return fmt.Errorf("unexpected final auth length %d", len(plain))
	}
	*stage = "derive_session_keys"
	sv1, sv2, err := d.AuthenticateEV2FirstPart2_block_3(plain)
	if err != nil {
		return err
	}
	if _, err = sam.SAMLoadInitVector(samav2.AES_ALG, make([]byte, 16)); err != nil {
		return err
	}
	enc, err := sam.SAMGenerateMAC(samav2.AES_ALG, sv1)
	if err != nil {
		return err
	}
	mac, err := sam.SAMGenerateMAC(samav2.AES_ALG, sv2)
	if err != nil {
		return err
	}
	return d.AuthenticateEV2FirstPart2_block_4(enc, mac)
}

func attempt(ctx context.Context, reader *cr100.Reader, sam *samav3.CsamAv3, c config, r *result) error {
	r.Stage = "detect"
	card, err := reader.ConnectCard()
	if err != nil {
		return err
	}
	uid, err := card.UID()
	if err != nil {
		return err
	}
	if len(uid) == 0 {
		return errors.New("no card UID in detection response")
	}
	r.UID = fmt.Sprintf("%X", uid)
	if card.SAK() != 0x20 {
		return fmt.Errorf("not an ISO14443-4 card: SAK %02X", card.SAK())
	}
	if !wait(ctx, c.settle) {
		return ctx.Err()
	}
	d := ev2.NewDesfire(readOnlyCard{card})
	r.Stage = "select_application"
	if err = d.SelectApplication([]byte{1, 0, 0}, nil); err != nil {
		return err
	}
	if c.selectOnly {
		r.Stage = "select_complete"
		r.OK = true
		return nil
	}
	if err = authenticate(d, sam, uid, c.cardKey, c.samEntry, &r.Stage); err != nil {
		return err
	}
	// Production layout: files 2=14 bytes, 3=9 bytes, 4=128 bytes.
	// Read file 4 in 48-byte chunks to match the existing CR100 workaround.
	for _, f := range []struct{ id, size int }{{2, 14}, {3, 9}, {4, 128}} {
		for offset := 0; offset < f.size; {
			n := f.size - offset
			if n > 48 {
				n = 48
			}
			r.Stage = fmt.Sprintf("read_file_%d_offset_%d_len_%d", f.id, offset, n)
			data, e := d.ReadData(f.id, ev2.TargetPrimaryApp, offset, n, ev2.FULL)
			if e != nil {
				return e
			}
			if len(data) != n {
				return fmt.Errorf("expected %d bytes, received %d", n, len(data))
			}
			offset += n
		}
	}
	r.Stage = "read_complete"
	r.OK = true
	return nil
}

func run() error {
	var c config
	flag.StringVar(&c.port, "port", "/dev/ttyACM0", "CR100 serial device")
	flag.IntVar(&c.baud, "baud", 115200, "serial baud rate")
	flag.StringVar(&c.samName, "sam-reader", "SAM", "substring selecting a unique PC/SC SAM reader")
	flag.IntVar(&c.samEntry, "sam-entry", 39, "SAM offline key entry (39 decimal = 0x27)")
	flag.IntVar(&c.cardKey, "card-key", 4, "DESFire application key number")
	flag.StringVar(&c.recovery, "recovery", "none", "none or poweroff; poweroff cycles NFC off/on after each attempt")
	flag.DurationVar(&c.duration, "duration", 2*time.Minute, "overall trial duration; a driver call can overrun this until it returns")
	flag.DurationVar(&c.pause, "interval", 150*time.Millisecond, "pause between attempts")
	flag.DurationVar(&c.settle, "settle", 0, "optional pause after UID detection, before select")
	flag.BoolVar(&c.list, "list-readers", false, "list PC/SC readers and exit without opening serial or authenticating")
	flag.BoolVar(&c.selectOnly, "select-only", false, "detect and select application only; no SAM access or file reads")
	flag.StringVar(&c.resetBefore, "reset-before", "none", "one reader recovery before trials: none, powercycle, posreset, mifarehalt")
	flag.BoolVar(&c.bounded, "bounded", true, "use cancelable read recovery for detection and APDU exchanges")
	flag.Parse()
	if c.resetBefore != "none" && c.resetBefore != "powercycle" && c.resetBefore != "posreset" && c.resetBefore != "mifarehalt" {
		return errors.New("invalid reset-before mode")
	}
	if c.recovery != "none" && c.recovery != "poweroff" {
		return errors.New("invalid recovery mode")
	}
	if c.duration <= 0 || c.pause < 0 || c.settle < 0 {
		return errors.New("invalid trial durations")
	}
	if c.cardKey < 0 || c.cardKey > 13 || c.samEntry < 0 || c.samEntry > 127 {
		return errors.New("key index out of range")
	}
	var sam *samav3.CsamAv3
	if !c.selectOnly || c.list {
		pc, err := pcsc.NewContext()
		if err != nil {
			return fmt.Errorf("PC/SC: %w", err)
		}
		defer pc.Release()
		names, err := pc.ListReaders()
		if err != nil {
			return err
		}
		for _, name := range names {
			log.Printf("PCSC_READER %q", name)
		}
		if c.list {
			return nil
		}
		matches := []string{}
		for _, name := range names {
			if strings.Contains(name, c.samName) {
				matches = append(matches, name)
			}
		}
		if len(matches) != 1 {
			return fmt.Errorf("expected exactly one SAM reader matching %q; found %d", c.samName, len(matches))
		}
		samCard, err := pcsc.NewReader(pc, matches[0]).ConnectCard()
		if err != nil {
			return err
		}
		defer samCard.DisconnectCard()
		sam = samav3.SamAV3(samCard)
		log.Print("SAM_READY host_auth=false")
	}
	serialPort, err := serial.OpenPort(&serial.Config{Name: c.port, Baud: c.baud, ReadTimeout: 30 * time.Millisecond})
	if err != nil {
		return err
	}
	defer serialPort.Close()
	dev := cr100.NewDevice(serialPort)
	reader := cr100.NewReaderWithDevice(dev)
	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(stopCtx, c.duration)
	defer cancel()
	if c.bounded {
		reader.EnableReadRecovery()
		reader.SetDetectionContext(ctx)
	}
	if c.resetBefore != "none" {
		if err := resetBeforeTrial(dev, c.resetBefore); err != nil {
			log.Printf("PROBE_RESET_ERROR mode=%s err=%v", c.resetBefore, err)
		}
	}
	log.Printf("PROBE_START read_only=true select_only=%t recovery=%s card_key=%d sam_entry=%d settle=%s duration=%s", c.selectOnly, c.recovery, c.cardKey, c.samEntry, c.settle, c.duration)
	enc := json.NewEncoder(os.Stderr)
	attempts, success := 0, 0
	for ctx.Err() == nil {
		attempts++
		start := time.Now()
		r := result{Time: start.Format(time.RFC3339Nano), Attempt: attempts, Recovery: c.recovery}
		if e := attempt(ctx, reader, sam, c, &r); e != nil {
			r.Error = e.Error()
		}
		r.ElapsedMS = time.Since(start).Milliseconds()
		if r.OK {
			success++
		}
		if err = enc.Encode(r); err != nil {
			return err
		}
		if c.recovery == "poweroff" && ctx.Err() == nil {
			cmd, e := sdk.PowerOffContactless(time.Second)
			if e == nil {
				_, e = dev.Transmit(cmd, time.Second)
			}
			if e == nil {
				var status int
				status, e = sdk.GetCmdId()
				if e == nil && status != sdk.CMD_SUC {
					e = fmt.Errorf("poweroff SDK status %02X", status)
				}
			}
			if e == nil {
				// powerOffNFC alone leaves this reader unable to detect again.
				// Explicitly reactivate the NFC field before the next poll.
				if !wait(ctx, 100*time.Millisecond) {
					break
				}
				cmd, e = sdk.PowerOnContactless(time.Second)
				if e == nil {
					_, e = dev.Transmit(cmd, 3*time.Second)
				}
				if e == nil {
					var status int
					status, e = sdk.GetCmdId()
					if e == nil && status != sdk.CMD_SUC {
						e = fmt.Errorf("poweron SDK status %02X", status)
					}
				}
			}
			log.Printf("PROBE_RECOVERY powercycle err=%v", e)
			if e != nil {
				return fmt.Errorf("poweroff recovery failed: %w", e)
			}
		}
		if !wait(ctx, c.pause) {
			break
		}
	}
	log.Printf("PROBE_SUMMARY select_only=%t attempts=%d successful_attempts=%d failures_or_no_card=%d", c.selectOnly, attempts, success, attempts-success)
	return nil
}
func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

// Reader control commands only. No card mutation commands are accepted here.
func resetBeforeTrial(dev *cr100.Device, mode string) error {
	exchange := func(label string, pack func() ([]byte, error), timeout time.Duration) error {
		cmd, err := pack()
		if err != nil {
			return err
		}
		_, err = dev.Transmit(cmd, timeout)
		status := -1
		if err == nil {
			status, err = sdk.GetCmdId()
			// ResetPosStatus acknowledges with command 0x20 (observed frame
			// 4D0001206C), unlike the 0x24 result of NFC commands.
			if err == nil && status != sdk.CMD_SUC && !(mode == "posreset" && status == 0x20) {
				err = fmt.Errorf("terminal status %02X", status)
			}
		}
		log.Printf("PROBE_RESET_STEP mode=%s step=%s status=%02X err=%v", mode, label, status, err)
		return err
	}
	switch mode {
	case "powercycle":
		offErr := exchange("nfc_off", func() ([]byte, error) { return sdk.PowerOffContactless(time.Second) }, time.Second)
		time.Sleep(200 * time.Millisecond)
		onErr := exchange("nfc_on", func() ([]byte, error) { return sdk.PowerOnContactless(time.Second) }, 3*time.Second)
		return errors.Join(offErr, onErr)
	case "posreset":
		return exchange("reset_pos_status", sdk.ResetPosStatus, 2*time.Second)
	case "mifarehalt":
		return exchange("mifare_halt", func() ([]byte, error) { return sdk.DoMifare(0x0e, time.Millisecond*10) }, time.Second)
	}
	return errors.New("unsupported reset")
}
