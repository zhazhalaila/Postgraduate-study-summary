package consensus

const (
	PBCOUTPUT = iota
	PRODUCER  = iota
	ABAOUTPUT = iota
	ABASTOP   = iota
)

type Epoch struct {
	path *Path
}
