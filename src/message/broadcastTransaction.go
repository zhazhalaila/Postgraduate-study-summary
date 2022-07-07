package message

type PBCModule struct {
	Round    int
	Initator int
}

type PrePrepare struct {
	Epoch            int
	Round            int
	Initiator        int
	TxsHash          [32]byte
	TxsHashSignature []byte
	// Using pointer to avoid memory copy
	Transactions *[][]byte
}

type Prepare struct {
	Epoch         int
	Round         int
	Initiator     int
	TxsHash       [32]byte
	VoteSignature []byte
	Voter         int
}
