package message

import "encoding/json"

type CBCEntrance struct {
	SpecificType uint8
	Candidate    int
	Payload      json.RawMessage
}

type CBCSEND struct {
	Candidate        int
	Qcs              [][]QuorumCert
	QcsHash          [32]byte
	QcsHashSignature []byte
}

type CBCACK struct {
	QcsHash       [32]byte
	VoteSignature []byte
	Voter         int
}

type CBCDONE struct {
	Candidate int
	QcsHash   [32]byte
	Proof     map[int][]byte
}

func GenCBCEntrance(specificType uint8, candidate int, payload []byte) CBCEntrance {
	cbcEntrance := CBCEntrance{
		SpecificType: specificType,
		Candidate:    candidate,
		Payload:      payload,
	}
	return cbcEntrance
}
