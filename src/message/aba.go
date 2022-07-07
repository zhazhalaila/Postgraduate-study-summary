package message

type ABAModule struct {
	ABAType      uint8
	LotteryTimes int
	Round        int
	Sender       int
	Payload      interface{}
}

type EST struct {
	// Estimate value
	BinValue int
}

type AUX struct {
	// Have seen 2f+1 times estimate value
	Element int
}

type CONF struct {
	// Have seen 2f+1 aux values
	Value int
}

type COIN struct {
	// Hash(LtteryId + Round)
	// Signature(HashMsg)
	HashMsg   []byte
	Signature []byte
}
