package message

type Decision struct {
	Producer  int
	Sender    int
	Delivered bool
	Proof     map[int][]byte
	Qcs       [][]QuorumCert
}

func GenDecision(producer, sender int, delivered bool, proof map[int][]byte, qcs [][]QuorumCert) Decision {
	d := Decision{
		Producer:  producer,
		Sender:    sender,
		Delivered: delivered,
		Proof:     proof,
		Qcs:       qcs,
	}
	return d
}
