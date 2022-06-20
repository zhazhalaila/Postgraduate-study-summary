package message

type PrePrepare struct {
	Epoch               int
	Round               int
	Initiator           int
	MerkleRoot          [32]byte
	MerkleRootSignature []byte
	// Using pointer to avoid memory copy
	Transactions *[][]byte
}

type Prepare struct {
	Epoch         int
	Round         int
	Initiator     int
	MerkleRoot    [32]byte
	VoteSignature []byte
	Voter         int
}
