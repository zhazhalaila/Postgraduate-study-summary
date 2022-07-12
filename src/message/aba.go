package message

import "encoding/json"

type ABAEntrance struct {
	ABAType      uint8
	LotteryTimes int
	Payload      json.RawMessage
}

type EST struct {
	Round  int
	Sender int
	// Estimate value
	BinValue int
}

type AUX struct {
	Round  int
	Sender int
	// Have seen 2f+1 times estimate value
	Element int
}

type CONF struct {
	Round  int
	Sender int
	// Have seen 2f+1 aux values
	Value int
}

type COIN struct {
	Round  int
	Sender int
	// Hash(LtteryId + Round)
	// Signature(HashMsg)
	HashMsg   []byte
	Signature []byte
}

func GenABAEntrance(specificType uint8, lotteryTimes int, payload []byte) ABAEntrance {
	abaEntrance := ABAEntrance{
		ABAType:      specificType,
		LotteryTimes: lotteryTimes,
		Payload:      payload,
	}
	return abaEntrance
}
