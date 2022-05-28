package message

type PrePrepare struct {
	Epoch               int
	Count               int
	MerkleRoot          []byte
	MerkleRootSignature []byte
	Initiator           int
	// Using pointer to avoid copy
	Transactions *[][]byte
}

type Prepare struct {
	Epoch         int
	Count         int
	MerkleRoot    []byte
	VoteSignature []byte
	Voter         int
}
