package message

const (
	PBType uint8 = iota
	ABAType
	NewTransactionsType
	SendType
	AckType
	DoneType
	PreprepareType
	PrepareType
)
