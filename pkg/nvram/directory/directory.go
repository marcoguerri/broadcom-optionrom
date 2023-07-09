package directory

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

type TypeSize uint32

func (t *TypeSize) Size() uint32 {
	return uint32(*t & 0x00FFFFFF)
}

func (t *TypeSize) Type() uint32 {
	return uint32(*t & 0xFF000000 >> 24)
}

func (t *TypeSize) SetSize(size uint32) {
	if size&0xFF000000>>24 != 0 {
		panic("size cannot be larger than 24 bits")
	}
	*t = TypeSize(uint32(*t)&0xFF000000 | size)

}

func (t *TypeSize) SetType(_type uint8) {
	*t = TypeSize(uint32(*t)&0x00FFFFFF | uint32(_type)<<24)
}

type Entry struct {
	Padding  uint32
	TypeSize TypeSize
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
