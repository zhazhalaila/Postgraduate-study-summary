package message

type ECHO struct {
	RootHash [32]byte
	Branch   *[][32]byte
	Shard    *[]byte
}

type RECOVERY struct {
	Producer int
	Sender   int
	Echos    map[int]map[int]ECHO
}

func GenRecovery(producer, sender int, echos map[int]map[int]ECHO) RECOVERY {
	r := RECOVERY{
		Producer: producer,
		Sender:   sender,
		Echos:    echos,
	}
	return r
}
