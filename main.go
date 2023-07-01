package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"strings"
)

type TypeSize uint32

type Entry struct {
	Padding  uint32
	TypeSize uint32
	Offset   uint32
}

func (e Entry) String() string {
	var s strings.Builder
	fmt.Fprintf(&s, "TypeSize: %x\n", e.TypeSize)
	fmt.Fprintf(&s, "Offset: %x\n", e.Offset)
	return s.String()
}

func (e Entry) GoString() string {
	return e.String()
}

type Directory struct {
	Entries [8]Entry
}

type Header struct {
	Magic      uint32
	Data       [4]uint32 // define what this data is
	Directory  Directory
	SingleByte uint8 // define what this is
	DirCRC     uint8
	Vpd        [86]uint8
	Crc        uint32

	bytes []byte
}

func (h *Header) FromReader(r io.Reader) error {

	headerSize := binary.Size(h)

	b := make([]byte, headerSize)

	err := binary.Read(bytes.NewBuffer(b), binary.BigEndian, headerSize)
	if err != nil {
		return fmt.Errorf("could not read NVRAM header: %s", err)
	}

	h.bytes = b
	return nil
}

func (h *Header) Bytes() []byte {
	return h.bytes
}

type NVRAM struct {
	header Header
	bytes  []byte
}

func (n *NVRAM) Bytes() []byte {
	return n.bytes
}

func (n *NVRAM) FromReader(r io.Reader) error {
	if err := n.header.FromReader(r); err != nil {
		return err
	}

	b, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("could not read data: %w", err)
	}

	n.bytes = b
	return nil

}

type directoryChecksum struct {
}

func (d directoryChecksum) check(nvram *NVRAM) error {
	return nil
}

// Check defines the interface for pre and post checks which are executed
// on the NVRAM
type Check interface {
	check(nvram *NVRAM) error
}

func main() {

	log.SetOutput(os.Stdout)

	flagnNvramBinPath := "nvramBinPath"
	flagNvramBinOut := "nvramBinOut"
	flagPxeOptionRomPath := "pxeOptionRomPath"

	nvramBinPath := flag.String(flagnNvramBinPath, "", "Path to the NVRAM dump")
	nvramBinOut := flag.String(flagNvramBinOut, "", "Output NVRAM binary, modified with the new OptionROM")
	pxeOptionRomPath := flag.String(flagPxeOptionRomPath, "", "Paht of the OptionROM to embed into the NVRAM binary")

	flag.Parse()

	if *nvramBinPath == "" || *nvramBinOut == "" || *pxeOptionRomPath == "" {
		log.Fatalf("%s ('%s'), %s ('%s'), %s ('%s') must all be set", flagnNvramBinPath, *nvramBinPath, flagNvramBinOut, *nvramBinOut, flagPxeOptionRomPath, *pxeOptionRomPath)
	}

	nvramOld, err := os.Open(*nvramBinPath)
	if err != nil {
		log.Fatalf("could not open old NVRAM binary file: %s", err)
	}

	nvram := NVRAM{}

	if err := nvram.FromReader(nvramOld); err != nil {
		log.Fatalf("coult not read NVRAM: %w", err)
	}

	checks := []Check{
		directoryChecksum{},
	}

	for _, check := range checks {
		if err := check.check(&nvram); err != nil {
			log.Fatalf("check failed: %w", err)
		}
	}

}
