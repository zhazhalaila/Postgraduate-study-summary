package message

type PrePrepare struct {
	Epoch               int
	Count               int
	MerkleRoot          [32]byte
	MerkleRootSignature []byte
	Initiator           int
	// Using pointer to avoid memory copy
	Transactions *[][]byte
}

type Prepare struct {
	Epoch         int
	Count         int
	MerkleRoot    [32]byte
	VoteSignature []byte
	Voter         int
}
