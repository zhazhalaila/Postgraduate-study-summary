package consensus

import "log"

type PBC struct {
	// Global log
	logger *log.Logger

	n  int
	f  int
	id int

	preparedThreshold int
	// using map to cache txs prevent Byzantine leader
	txs map[[32]byte]*[][]byte
	// collect all voters' signatures
	sigs map[[32]byte]map[int][]byte
}
