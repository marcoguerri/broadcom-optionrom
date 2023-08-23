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

func (d directoryChecksum) set(nvram *nvram.NVRAM) error {

	buff := new(bytes.Buffer)
	if err := binary.Write(buff, binary.BigEndian, &nvram.Header); err != nil {
		return fmt.Errorf("could not write header: %w", err)
	}

	directoryBytes, err := nvram.GetRange(nvrange.DirectoryCrcRange)
	if err != nil {
		return fmt.Errorf("could not get directory bytes: %w", err)
	}

	newCrcDir := crc.Crc8(directoryBytes)

	nvram.Header.DirCRC = newCrcDir
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

func (v vpdChecksum) set(nvram *nvram.NVRAM) error {

	vpdBytes, err := nvram.GetRange(nvrange.VpdCrcRange)
	if err != nil {
		return fmt.Errorf("could not get vpd bytes: %w", err)
	}

	crc := crc.Crc32(vpdBytes)
	nvram.Header.Crc = uint32(crc.Reverse())
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
	return fmt.Errorf("no entry of type 0 with size != 0 found")
}

func (r romChecksum) set(nvram *nvram.NVRAM) error {
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

			rangeRomChecksum := nvrange.Range{
				Start:  rangeRom.Start + rangeRom.Length - 4,
				Length: 4,
			}
			buff := new(bytes.Buffer)
			if err := binary.Write(buff, binary.BigEndian, newCrc.Reverse()); err != nil {
				return fmt.Errorf("could not serialize new crc for OptionROM: %w", err)
			}
			return nvram.SetRange(rangeRomChecksum, buff.Bytes())
		}
	}

	return fmt.Errorf("no entry of type 0 with size != 0 found")
}

type romBinary struct {
	rom []byte
}

func (r romBinary) set(nvram *nvram.NVRAM) error {
	for index := range nvram.Header.Directory.Entries {
		entry := &nvram.Header.Directory.Entries[index]
		if entry.TypeSize.Type() == 0 && entry.TypeSize.Size() != 0 {
			rangeRom := nvrange.Range{
				Start:  entry.Offset,
				Length: uint32(len(r.rom)),
			}
			err := nvram.SetRange(rangeRom, r.rom)
			if err != nil {
				return fmt.Errorf("could not set NVRAM bytes: %w", err)
			}
			entry.TypeSize.SetSize(uint32(len(r.rom)))
			return nil
		}
	}
	return fmt.Errorf("no entry of type 0 with size != 0 found")
}

// Check defines the interface for pre and post checks does are executed
// on the NVRAM
type Check interface {
	check(nvram *nvram.NVRAM) error
}

type Set interface {
	set(nvram *nvram.NVRAM) error
}

func main() {

	var (
		inNvramBinPath, outNvramBinPath, uefiOptionRomPath string
	)

	debug := flag.Bool("debug", false, "Enable debug logging")

	checkCmd := flag.NewFlagSet("check", flag.ExitOnError)
	checkCmd.StringVar(&inNvramBinPath, "inNvramBinPath", "", "Path of the NVRAM binary dump")

	writeOptionRomCmd := flag.NewFlagSet("writeOptionRom", flag.ExitOnError)
	writeOptionRomCmd.StringVar(&inNvramBinPath, "inNvramBinPath", "", "Path of the input NVRAM binary dump")
	writeOptionRomCmd.StringVar(&outNvramBinPath, "outNvramBinPath", "", "Path of the output NVRAM binary dump")
	writeOptionRomCmd.StringVar(&uefiOptionRomPath, "uefiOptionRomPath", "", "Path of the OptionROM binary to write to NVRAM")

	flagSets := []*flag.FlagSet{checkCmd, writeOptionRomCmd}

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\n=== Supported commands ===\n")
		for _, c := range flagSets {
			fmt.Fprintf(flag.CommandLine.Output(), "-> Command %s:\n", c.Name())
			c.PrintDefaults()
		}
	}

	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatalf("command required")
		flag.Usage()
		os.Exit(1)
	}
	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	flagSetArgs := os.Args[flag.NFlag()+2:]
	command := os.Args[flag.NFlag()+1]

	var flagSet *flag.FlagSet

	switch command {
	case "check":
		flagSet = checkCmd
	case "writeOptionRom":
		flagSet = writeOptionRomCmd
	}

	if flagSet == nil {
		log.Fatalf("unsupported command %s", command)
	}

	flagSet.Parse(flagSetArgs)
	flagSet.VisitAll(func(f *flag.Flag) {
		if f.Value.String() == "" {
			log.Fatalf("%s is required for command %s", f.Name, command)
		}
	})

	switch command {

	case "check":

		nvramOld, err := os.Open(inNvramBinPath)
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
		fmt.Println("OK")

	case "writeOptionRom":

		optRom, err := os.ReadFile(uefiOptionRomPath)
		if err != nil {
			log.Fatalf("could not read UEFI OptionROM path")
		}

		sets := []Set{
			romBinary{rom: optRom},
			directoryChecksum{},
			vpdChecksum{},
			romChecksum{},
		}

		nvramOld, err := os.Open(inNvramBinPath)
		if err != nil {
			log.Fatalf("could not open old NVRAM binary file: %s", err)
		}

		nvram := nvram.NVRAM{}

		if err := nvram.FromReader(nvramOld); err != nil {
			log.Fatalf("coult not read NVRAM: %v", err)
		}

		for _, set := range sets {
			if err := set.set(&nvram); err != nil {
				log.Fatalf("could not run setter: %v", err)
			}
		}

		newNvram := new(bytes.Buffer)
		_, err = nvram.WriteTo(newNvram)
		if err != nil {
			log.Fatalf("could not serialize new NVRAM")
		}

		if err = os.WriteFile(outNvramBinPath, newNvram.Bytes(), 0644); err != nil {
			log.Fatalf("could not write new NVRAM binary")
		}

		fmt.Println("OK")

	}
	//set := []Set{
	//	directoryChecksum{}
	//
}
