package message

type NewTransaction struct {
	ClientAddr   string
	Transactions *[][]byte
}
