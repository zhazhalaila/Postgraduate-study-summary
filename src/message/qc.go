package message

type QuorumCert struct {
	Round      int
	Initiator  int
	RootHash   [32]byte
	Signatures map[int][]byte
}
