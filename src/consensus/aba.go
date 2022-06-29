package consensus

import (
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1"
)

// with non buffer channel

type ABA struct {
	// Global log
	logger *log.Logger

	n         int
	f         int
	id        int
	epoch     int
	lotteries int
	round     int

	// Already decided
	alreadyDecide *int

	binValues  map[int][]int
	estValues  map[int]map[int][]int
	auxValues  map[int]map[int][]int
	confValues map[int]map[int][]int

	// Est values send status
	estSent map[int]map[int]bool

	// Coin shares
	coinShares map[int]map[int][]byte

	// public key
	pubKey *secp256k1.PublicKey
	// private key
	priKey *secp256k1.PrivateKey
	// public key for all peers
	pubKeys map[int]*secp256k1.PublicKey

	event chan struct{}
}
