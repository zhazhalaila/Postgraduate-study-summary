package message

type QuorumCert struct {
	Round      int
	Initiator  int
	TxsHash    [32]byte
	Signatures map[int][]byte
}
