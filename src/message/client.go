package message

type NewTransaction struct {
	ClientAddr   string
	Epoch        int
	Round        int
	Transactions *[][]byte
}
