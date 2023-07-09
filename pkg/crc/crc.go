package crc

type TCrc32 uint32

func (c TCrc32) Reverse() TCrc32 {

	return c&0xFF000000>>24 | c&0x00FF0000>>8 | c&0x0000FF00<<8 | c&0x00000FF<<24

}

func Crc8(b []byte) uint8 {
	var param2 int32
	var bVar uint8

	size := len(b)
	for param2 = 0; param2 < int32(size); param2++ {
		bVar += uint8(b[param2])
	}

	// TODO: explain what this is euivalent to
	return (bVar ^ 0x000000ff) + 1
}

func Crc32(b []byte) TCrc32 {

	var unaff_EBX uint32
	unaff_EBX = 0xFFFFFFFF
	var uVar3 uint32
	var uVar4 uint32

	for uVar4 = 0; uVar4 < uint32(len(b)); uVar4++ {
		var uVar2 uint32
		uVar2 = uint32(b[uVar4])

		for uVar1 := 0; uVar1 < 8; uVar1++ {
			if ((unaff_EBX ^ uint32(uVar2)) & 1) == 0 {
				uVar3 = 0
			} else {
				uVar3 = 0xedb88320
			}
			unaff_EBX = unaff_EBX>>1 ^ uVar3
			uVar2 = uVar2 >> 1
		}
	}

	return TCrc32(unaff_EBX ^ 0xFFFFFFFF)
}
