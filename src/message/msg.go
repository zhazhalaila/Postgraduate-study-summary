package message

const (
	PBType uint8 = iota
	ABAType

	// PB
	NewTransactionsType
	SendType
	AckType
	DoneType

	// ABA
)
