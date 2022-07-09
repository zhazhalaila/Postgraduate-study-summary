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
	Transactions *[][]byte
}

type SEND struct {
	Initiator         int
	RootHash          [32]byte
	RootHashSignature []byte
	Branch            *[][32]byte
	Share             *[]byte
}

type ACK struct {
	RootHash      [32]byte
	VoteSignature []byte
	Voter         int
}

type DONE struct {
	Initiator  int
	RootHash   [32]byte
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
