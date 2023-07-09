package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"

	"broadcom-pxe/pkg/crc"
	"broadcom-pxe/pkg/nvram"
	"broadcom-pxe/pkg/nvram/nvrange"

	log "github.com/sirupsen/logrus"
)

type directoryChecksum struct {
}

func (d directoryChecksum) check(nvram *nvram.NVRAM) error {

	buff := new(bytes.Buffer)
	if err := binary.Write(buff, binary.BigEndian, &nvram.Header); err != nil {
		return fmt.Errorf("could not write header: %w", err)
	}

	directoryBytes, err := nvram.GetRange(nvrange.DirectoryCrcRange)
	if err != nil {
		return fmt.Errorf("could not get directory bytes: %w", err)
	}

	oldCrcRaw, err := nvram.GetRange(nvrange.DirectoryCrcLocation)
	if err != nil {
		return fmt.Errorf("could not get existing directory CRC: %w", err)
	}

	var oldCrcDir uint8
	if err := binary.Read(bytes.NewBuffer(oldCrcRaw), binary.BigEndian, &oldCrcDir); err != nil {
		return fmt.Errorf("could not interpret CRC as 1 byte big endian number: %w", err)
	}

	newCrcDir := crc.Crc8(directoryBytes)
	if oldCrcDir != newCrcDir {
		return fmt.Errorf("existing directory crc (0x%x) mismatches calculated directory crc (0x%x)", oldCrcDir, newCrcDir)
	}

	return nil
}

type vpdChecksum struct {
}

func (v vpdChecksum) check(nvram *nvram.NVRAM) error {

	vpdBytes, err := nvram.GetRange(nvrange.VpdCrcRange)
	if err != nil {
		return fmt.Errorf("could not get vpd bytes: %w", err)
	}

	crc := crc.Crc32(vpdBytes)

	if nvram.Header.Crc != uint32(crc.Reverse()) {
		return fmt.Errorf("existing CRC (%x) does not match VPD crc (%x)", nvram.Header.Crc, crc.Reverse())

	}

	return nil
}

type romChecksum struct {
}

func (r romChecksum) check(nvram *nvram.NVRAM) error {
	for _, entry := range nvram.Header.Directory.Entries {
		if entry.TypeSize.Type() == 0 && entry.TypeSize.Size() != 0 {
			rangeRom := nvrange.Range{
				Start:  entry.Offset,
				Length: entry.TypeSize.Size() * 4,
			}

			romBytes, err := nvram.GetRange(rangeRom)
			if err != nil {
				return fmt.Errorf("could not extract OptionROM bytes: %w", err)
			}
			newCrc := crc.Crc32(romBytes[:rangeRom.Length-4])

			var oldCrc uint32
			if err := binary.Read(bytes.NewBuffer(romBytes[rangeRom.Length-4:]), binary.BigEndian, &oldCrc); err != nil {
				return fmt.Errorf("could not read OptionROM CRC: %v", err)
			}

			if uint32(newCrc.Reverse()) != oldCrc {
				return fmt.Errorf("existing CRC (%x) does not match new CRC (%x)", oldCrc, newCrc.Reverse())
			}
			return nil

		}
	}
	return nil
}

// Check defines the interface for pre and post checks which are executed
// on the NVRAM
type Check interface {
	check(nvram *nvram.NVRAM) error
}

type Set interface {
	set(nvram *nvram.NVRAM) error
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

	nvram := nvram.NVRAM{}

	if err := nvram.FromReader(nvramOld); err != nil {
		log.Fatalf("coult not read NVRAM: %v", err)
	}

	checks := []Check{
		directoryChecksum{},
		vpdChecksum{},
		romChecksum{},
	}

	for _, check := range checks {
		if err := check.check(&nvram); err != nil {
			log.Fatalf("check failed: %v", err)
		}
	}

	//set := []Set{
	//	directoryChecksum{}
	//
}
