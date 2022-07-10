package message

const (
	PBType uint8 = iota
	CBCType
	ABAType

	// PB
	NewTransactionsType
	SendType
	AckType
	DoneType

	// CBC
	CBCSendType
	CBCAckType
	CBCDoneType

	// ABA
)
