package message

type PBCWrapper struct {
	Msg interface{}
}

type PrePrepare struct {
	Epoch               int
	Count               int
	MerkleRoot          [32]byte
	MerkleRootSignature []byte
	Initiator           int
	// Using pointer to avoid copy
	Transactions *[][]byte
}

type Prepare struct {
	Epoch         int
	Count         int
	MerkleRoot    [32]byte
	VoteSignature []byte
	Voter         int
}
