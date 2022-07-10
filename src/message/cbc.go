package message

import "encoding/json"

type CBCEntrance struct {
	SpecificType uint8
	Round        int
	Candidate    int
	Payload      json.RawMessage
}

type CBCSEND struct {
	Candidate        int
	Qcs              *[][]QuorumCert
	QcsHash          []byte
	QcsHashSignature []byte
}

type CBCACK struct {
	QcsHash       []byte
	VoteSignature []byte
	Voter         int
}

type CBCDONE struct {
	Candidate int
	QcsHash   []byte
	Proof     map[int][]byte
}

func GenCBCEntrance(specificType uint8, round, candidate int, payload []byte) CBCEntrance {
	cbcEntrance := CBCEntrance{
		SpecificType: specificType,
		Round:        round,
		Candidate:    candidate,
		Payload:      payload,
	}
	return cbcEntrance
}
