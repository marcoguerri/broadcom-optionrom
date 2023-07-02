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

func crc8(b []byte) uint8 {
    var param2 int32
    var bVar uint8

	size := len(b)
    for param2 = 0; param2 < int32(size); param2++ {
        bVar += uint8(b[param2])
    }

	// TODO: explain what this is euivalent to
    return (bVar ^ 0x000000ff) + 1
}

func crc32(b []byte) uint32 {

    var unaff_EBX uint32
    unaff_EBX = 0xFFFFFFFF
    var uVar3 uint32
    var uVar4 uint32

    for uVar4 = 0 ; uVar4 < uint32(len(b)); uVar4++ {
       var uVar2 uint32
       uVar2 = uint32(b[uVar4])


       for uVar1:=0; uVar1<8; uVar1++ {
               if (((unaff_EBX ^ uint32(uVar2)) & 1) == 0) {
                   uVar3=0
                } else {
                    uVar3=0xedb88320;
                }
                unaff_EBX = unaff_EBX >> 1 ^ uVar3;
                uVar2 = uVar2 >> 1;
        }
	}

	return unaff_EBX^0xFFFFFFFF
}


type Range struct {
	Start uint32
	Length uint32
}


var directoryCrcRange = Range{
	Start: 20,
	Length: 96,
}


var directoryCrcLocation = Range {
	Start: 117,
	Length: 1,
}


var vpdCrcRange = Range {
	Start: 116,
	Length: 136,
}


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
	Vpd        [134]uint8
	Crc        uint32
}

func (h *Header) FromReader(r io.Reader) error {

	err := binary.Read(r, binary.BigEndian, h)
	if err != nil {
		return fmt.Errorf("could not read NVRAM header: %s", err)
	}

	return nil
}


type NVRAM struct {
	Header Header
	Data  []byte
}


func (n *NVRAM) GetRange(r Range) ([]byte, error) {
	headerSize := uint32(binary.Size(n.Header))
	totalSize := headerSize + uint32(len(n.Data))

	if r.Start + r.Length > totalSize {
		return nil, fmt.Errorf("r.Start (%d) + r.Length (%d) = (%d) > len(NVRAM) (%d)", r.Start, r.Length, r.Start+r.Length, totalSize)
	}

	// Range starts in the header. There is a limitation that the 
    // range must stay within the header. This could probably be 
    // removed.
	if r.Start <  headerSize {
		if r.Start + r.Length > headerSize {
			return nil, fmt.Errorf("range starts within header size and must stay within header size (%d > %d)", r.Start+r.Length, headerSize)
		}
    	buff := new(bytes.Buffer)
    	if err := binary.Write(buff, binary.BigEndian, &n.Header); err != nil {
        	return nil, fmt.Errorf("could not write header: %w", err)
    	}
		return buff.Bytes()[r.Start:r.Start+r.Length], nil
	}

	// Range starts in the data section. We already checked that the overall
	// lenght is within the NVRAM range
	return n.Data[headerSize+r.Start: headerSize+r.Start+r.Length], nil
}


func (n *NVRAM) FromReader(r io.Reader) error {
	if err := n.Header.FromReader(r); err != nil {
		return err
	}

	b, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("could not read data: %w", err)
	}

	n.Data = b
	return nil

}

type directoryChecksum struct {
}

func (d directoryChecksum) check(nvram *NVRAM) error {

    buff := new(bytes.Buffer)
    if err := binary.Write(buff, binary.BigEndian, &nvram.Header); err != nil {
        return fmt.Errorf("could not write header: %w", err)
    }


	directoryBytes, err := nvram.GetRange(directoryCrcRange)
	if err != nil {
		return fmt.Errorf("could not get directory bytes: %w", err)
	}

	oldCrcRaw, err := nvram.GetRange(directoryCrcLocation)
	if err != nil {
		return fmt.Errorf("could not get existing directory CRC: %w", err)
	}

	var oldCrcDir uint8 
	if err := binary.Read(bytes.NewBuffer(oldCrcRaw), binary.BigEndian, &oldCrcDir); err != nil {
		return fmt.Errorf("could not interpret CRC as 1 byte big endian number: %w", err)
	}

	newCrcDir := crc8(directoryBytes)
	if oldCrcDir != newCrcDir {
		return fmt.Errorf("existing directory crc (0x%x) mismatches calculated directory crc (0x%x)", oldCrcDir, newCrcDir)
	}

	return nil
}


type vpdChecksum struct {
}


func (v vpdChecksum) check(nvram *NVRAM) error {

	vpdBytes, err := nvram.GetRange(vpdCrcRange)
	if err != nil {
		return fmt.Errorf("could not get vpd bytes: %w", err)
	}

	crc := crc32(vpdBytes)

	crcLe := (crc & 0xFF << 24) | (crc & 0xFF00 << 8) | (crc & 0xFF0000 >> 8) | (crc & 0xFF000000 >> 24)

	if nvram.Header.Crc != crcLe {
		return fmt.Errorf("existing CRC (%x) does not match VPD crc (%x)", nvram.Header.Crc, crcLe)

	}

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
		vpdChecksum{},
	}

	for _, check := range checks {
		if err := check.check(&nvram); err != nil {
			log.Fatalf("check failed: %w", err)
		}
	}

}
