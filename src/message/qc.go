package message

type QuorumCert struct {
	Initiator  int
	Round      int
	RootHash   [32]byte
	Signatures map[int][]byte
}

type Path struct {
	Epoch     int
	Candidate int
	Qcs       [][]QuorumCert
}
