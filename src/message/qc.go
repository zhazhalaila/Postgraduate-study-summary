package message

type QuorumCert struct {
	Signatures [][]byte
	Epoch      int
	R          int
	Proposer   int
	Hash       []byte
}
