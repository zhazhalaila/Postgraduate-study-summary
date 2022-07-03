package consensus

import (
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
)

// with non buffer channel

type ABA struct {
	// Global log
	logger    *log.Logger
	transport *libnet.NetworkTransport

	n         int
	f         int
	id        int
	epoch     int
	lotteries int
	round     int

	// Already decided
	alreadyDecide *int

	// Cache bin, est, aux, conf values for each epoch
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

	input chan int
	event chan struct{}
}

func MakeABA(
	logger *log.Logger,
	transport *libnet.NetworkTransport,
	n, f, id, epoch, lotteries, round int,
	pubKey *secp256k1.PublicKey,
	priKey *secp256k1.PrivateKey,
	pubKeys map[int]*secp256k1.PublicKey,
) *ABA {
	aba := &ABA{}
	aba.logger = logger
	aba.transport = transport
	aba.n = n
	aba.f = f
	aba.id = id
	aba.epoch = epoch
	aba.lotteries = lotteries
	aba.round = 0
	aba.pubKey = pubKey
	aba.priKey = priKey
	aba.pubKeys = pubKeys
	aba.input = make(chan int)
	aba.event = make(chan struct{})
	go aba.run()
	return aba
}

func (aba *ABA) run() {
	// Wait for
	// est := <-aba.input
	<-aba.input

	for {
		// Wait for binary values not null
		for range aba.event {
			if len(aba.binValues[aba.round]) != 0 {
				break
			}
		}
		// Broadcast binary value

	}
}
