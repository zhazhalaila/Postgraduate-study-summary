package consensus

import (
	"log"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	"github.com/zhazhalaila/PipelineBFT/src/message"
	"github.com/zhazhalaila/PipelineBFT/src/verify"
)

const (
	Both = iota
)

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

	// estimate value
	est int

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

	input  chan int
	stop   chan struct{}
	exit   chan struct{}
	event  chan struct{}
	coinCh chan int
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
	aba.stop = make(chan struct{})
	aba.exit = make(chan struct{})
	aba.event = make(chan struct{})
	aba.coinCh = make(chan int, 20)
	go aba.recv()
	go aba.run()
	return aba
}

// Chech element in slice
func inSlice(s int, list []int) bool {
	for _, b := range list {
		if b == s {
			return true
		}
	}
	return false
}

// ABA receive msg handler
func (aba *ABA) recv() {
Loop:
	for {
		select {
		case <-aba.stop:
			break Loop
		case <-time.After(2 * time.Minute):
			break Loop
		}
	}

	close(aba.exit)
}

// ABA event handler
func (aba *ABA) run() {
	// Wait for input value
	aba.est = <-aba.input

	if ok := aba.estSent[aba.round][aba.est]; ok {
		// Has broadcasted est value
	} else {
		// Broadcast est value
	}

	for {
		// Wait for binary values not null
	WaitBinary:
		for {
			select {
			case <-aba.exit:
				return
			case <-aba.event:
				if len(aba.binValues[aba.round]) != 0 {
					break WaitBinary
				}
			}
		}
		// Wait for receive 2f+1 aux msg
	AuxThreshold:
		for {
			select {
			case <-aba.exit:
				return
			case <-aba.event:
				if ok := aba.auxThreshold(aba.round); ok {
					break AuxThreshold
				}
			}
		}

		// Broadcast binary value

		// Wait for receive 2f+1 conf msg
		var values int
	ConfThreshold:
		for {
			select {
			case <-aba.exit:
				return
			case <-aba.event:
				v, ok := aba.confThreshold(aba.round)
				if ok {
					values = v
					break ConfThreshold
				}
			}
		}
		var coin int
		select {
		case <-aba.exit:
			return
		case coin = <-aba.coinCh:
		}

		if stop := aba.setNewEst(values, coin); stop {
			break
		} else {
			aba.round++
			// Broadcast est value
		}
	}
}

// Set new estimate value
func (aba *ABA) setNewEst(values int, coin int) bool {
	stop := false
	if values != Both {
		if values == coin {
			if aba.alreadyDecide == nil {
				aba.alreadyDecide = &values
				select {
				case <-aba.exit:
					return stop
				default:
					// notify epoch current ABA was done
				}
			} else if *aba.alreadyDecide == values {
				stop = true
			}
		}
		aba.est = values
	} else {
		aba.est = coin
	}

	return false
}

// If receive >= 2f+1 valid aux msg, broadcast conf value
func (aba *ABA) auxThreshold(round int) bool {
	// If receive >= 2f+1 aux msg with 1, broadcast (1.)
	if inSlice(1, aba.binValues[round]) && len(aba.auxValues[round][1]) >= aba.n-aba.f {
		// Broadcast Conf value (1,)
		return true
	}

	// If receive >= 2f+1 aux msg with 0, broadcast (0.)
	if inSlice(0, aba.binValues[round]) && len(aba.auxValues[round][0]) >= aba.n-aba.f {
		// Broadcast Conf value (0,)
		return true
	}

	// If receive >= 2f+1 aux msg with 0 & 1, broadcast (0,1)
	count := 0
	for _, v := range aba.binValues[round] {
		count += len(aba.auxValues[round][v])
	}
	if count >= aba.n-aba.f {
		// Broadcast Conf value (0, 1)
		return true
	}
	return false
}

// If receive >= 2f+1 valid aux msg, broadcast coin share
func (aba *ABA) confThreshold(round int) (int, bool) {
	// If receive >= 2f+1 conf msg with 1, set value to 1 for current round
	if inSlice(1, aba.binValues[round]) && len(aba.confValues[round][1]) >= aba.n-aba.f {
		// Broadcast coin share
		return 1, true
	}

	// If receive >= 2f+1 conf msg with 0, set value to 0 for curent round
	if inSlice(0, aba.binValues[round]) && len(aba.confValues[round][0]) >= aba.n-aba.f {
		// Broadcast coin share
		return 0, true
	}

	// If receive >= 2f+1 conf msg
	// len(conf[(0)]) + len(conf[(1)]) + len(conf[(0,1)]) >= 2f+1, set value to 2
	// len(conf[(0)]) + len(conf[(1)]) || len(conf[(0)]) + len(conf[(0,1)]) || len(conf[(1)]) + len(conf[(0,1)])
	count := 0

	if len(aba.binValues[round]) == 2 {
		count += len(aba.confValues[round][Both])
		count += len(aba.confValues[round][0])
		count += len(aba.confValues[round][1])
	}

	if count >= aba.n-aba.f {
		// Broadcast coin share
		return Both, true
	}

	return -1, false
}

// Upon receive est msg from other nodes, do this:
// If receive f+1 same est value, broadcast est
// If receive 2f+1 same est value, add to binary value
func (aba *ABA) handleEST(est message.EST, round, sender int) {
	if ok := inSlice(sender, aba.estValues[round][est.BinValue]); ok {
		return
	}

	aba.estValues[round][est.BinValue] = append(aba.estValues[round][est.BinValue], sender)

	estCount := len(aba.estValues[round][est.BinValue])
	estSent := aba.estSent[round][est.BinValue]

	if estCount == aba.f+1 && !estSent {
		// Broadcast new est value
	}

	if estCount == 2*aba.f+1 {
		aba.binValues[round] = append(aba.binValues[round], est.BinValue)
		aba.event <- struct{}{}
	}
}

func (aba *ABA) handleAUX(aux message.AUX, round, sender int) {
	if ok := inSlice(sender, aba.auxValues[round][aux.Element]); ok {
		return
	}

	aba.auxValues[round][aux.Element] = append(aba.auxValues[round][aux.Element], sender)
	aba.event <- struct{}{}
}

func (aba *ABA) handleCONF(conf message.CONF, round, sender int) {
	if ok := inSlice(sender, aba.confValues[round][conf.Value]); ok {
		return
	}

	aba.confValues[round][conf.Value] = append(aba.confValues[round][conf.Value], sender)
	aba.event <- struct{}{}
}

func (aba *ABA) handleCOIN(coin message.COIN, round, sender int) {
	err := verify.VerifySignature(coin.HashMsg, coin.Signature, aba.pubKeys, sender)
	if err != nil {
		return
	}

	if len(aba.coinShares[round]) > aba.f+1 {
		return
	}

	if _, ok := aba.coinShares[round][sender]; ok {
		return
	}

	aba.coinShares[round][sender] = coin.Signature
	if len(aba.coinShares[round]) == aba.f+1 {
		aba.coinCh <- int(coin.HashMsg[0]) % 2
	}
}
