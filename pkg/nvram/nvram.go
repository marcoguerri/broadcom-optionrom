package nvram

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"broadcom-pxe/pkg/nvram/directory"
	nvramrange "broadcom-pxe/pkg/nvram/nvrange"
)

type NVRAM struct {
	Header directory.Header
	Data   []byte
}

func (n *NVRAM) GetRange(r nvramrange.Range) ([]byte, error) {
	headerSize := uint32(binary.Size(n.Header))
	totalSize := headerSize + uint32(len(n.Data))

	if r.Start+r.Length > totalSize {
		return nil, fmt.Errorf("r.Start (%d) + r.Length (%d) = (%d) > len(NVRAM) (%d)", r.Start, r.Length, r.Start+r.Length, totalSize)
	}

	// Range starts in the header. There is a limitation that the
	// range must stay within the header. This could probably be
	// removed.
	if r.Start < headerSize {
		if r.Start+r.Length > headerSize {
			return nil, fmt.Errorf("range starts within header size and must stay within header size (%d > %d)", r.Start+r.Length, headerSize)
		}
		buff := new(bytes.Buffer)
		if err := binary.Write(buff, binary.BigEndian, &n.Header); err != nil {
			return nil, fmt.Errorf("could not write header: %w", err)
		}
		return buff.Bytes()[r.Start : r.Start+r.Length], nil
	}

	// Range starts in the data section. We already checked that the overall
	// lenght is within the NVRAM range
	return n.Data[r.Start-headerSize : r.Start-headerSize+r.Length], nil
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
