package message

import "encoding/json"

type PBEntrance struct {
	SpecificType uint8
	Round        int
	Initiator    int
	Payload      json.RawMessage
}

type NewTransaction struct {
	ClientAddr   string
	Transactions [][]byte
}

type SEND struct {
	Initiator int
	Txs       [][]byte
	TxsHash   [32]byte
	Signature []byte
}

type ACK struct {
	TxsHash       [32]byte
	VoteSignature []byte
	Voter         int
}

type DONE struct {
	Initiator  int
	TxsHash    [32]byte
	Signatures map[int][]byte
}

func GenPBEntrance(specificType uint8, round, initator int, payload []byte) PBEntrance {
	pbEntrance := PBEntrance{
		SpecificType: specificType,
		Round:        round,
		Initiator:    initator,
		Payload:      payload,
	}
	return pbEntrance
}
