package nvrange

type Range struct {
	Start  uint32
	Length uint32
}

var DirectoryCrcRange = Range{
	Start:  20,
	Length: 96,
}

var DirectoryCrcLocation = Range{
	Start:  117,
	Length: 1,
}

var VpdCrcRange = Range{
	Start:  116,
	Length: 136,
}
