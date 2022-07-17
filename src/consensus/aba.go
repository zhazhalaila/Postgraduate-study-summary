package consensus

import (
	"crypto/sha256"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/sasha-s/go-deadlock"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	"github.com/zhazhalaila/PipelineBFT/src/message"
	"github.com/zhazhalaila/PipelineBFT/src/verify"
)

const (
	Both = -1
)

type ABA struct {
	mu deadlock.Mutex
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

	epochEvent chan Event

	inCh   chan message.ABAEntrance
	input  chan int
	stop   chan struct{}
	done   chan struct{}
	exit   chan struct{}
	event  chan struct{}
	coinCh chan int
}

func MakeABA(
	logger *log.Logger,
	transport *libnet.NetworkTransport,
	n, f, id, epoch, lotteries int,
	pubKey *secp256k1.PublicKey,
	priKey *secp256k1.PrivateKey,
	pubKeys map[int]*secp256k1.PublicKey,
	epochEvent chan Event) *ABA {
	aba := &ABA{}
	aba.logger = logger
	aba.transport = transport
	aba.n = n
	aba.f = f
	aba.id = id
	aba.epoch = epoch
	aba.lotteries = lotteries
	aba.round = 0
	aba.alreadyDecide = nil

	aba.binValues = make(map[int][]int)
	aba.estValues = make(map[int]map[int][]int)
	aba.auxValues = make(map[int]map[int][]int)
	aba.confValues = make(map[int]map[int][]int)
	aba.estSent = make(map[int]map[int]bool)
	aba.coinShares = make(map[int]map[int][]byte)

	aba.pubKey = pubKey
	aba.priKey = priKey
	aba.pubKeys = pubKeys
	aba.epochEvent = epochEvent

	aba.inCh = make(chan message.ABAEntrance, aba.n*aba.n*aba.n)
	aba.input = make(chan int)
	aba.stop = make(chan struct{})
	aba.done = make(chan struct{})
	aba.exit = make(chan struct{})
	aba.event = make(chan struct{}, aba.n*4)
	aba.coinCh = make(chan int, 20)
	go aba.recv()
	go aba.run()
	return aba
}

func (aba *ABA) InputEST(est int) {
	aba.logger.Printf("[Epoch:%d] [LotteryTimes:%d] ABA start.\n", aba.epoch, aba.lotteries)
	aba.input <- est
}

func (aba *ABA) InputValue(msg message.ABAEntrance) {
	aba.inCh <- msg
}

func (aba *ABA) Stop() {
	aba.logger.Printf("[Epoch:%d] [ABAInstance:%d] close.\n", aba.epoch, aba.lotteries)
	close(aba.stop)
}

func (aba *ABA) Done() <-chan struct{} {
	return aba.done
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
		// Not timeout, just kill goroutine after long time not see other's msg(other peers exit ABA)
		case <-time.After(2 * time.Minute):
			break Loop
		case msg := <-aba.inCh:
			aba.handleMsg(msg)
		}
	}
	aba.logger.Printf("[Epoch:%d] [ABAInstanceId:%d] receive stop from epoch.\n", aba.epoch, aba.lotteries)
	close(aba.exit)
}

func (aba *ABA) handleMsg(entrance message.ABAEntrance) {
	switch entrance.ABAType {
	case message.EstType:
		var est message.EST
		err := json.Unmarshal(entrance.Payload, &est)
		if err != nil {
			return
		}
		aba.handleEST(est)

	case message.AuxType:
		var aux message.AUX
		err := json.Unmarshal(entrance.Payload, &aux)
		if err != nil {
			return
		}
		aba.handleAUX(aux)

	case message.ConfType:
		var conf message.CONF
		err := json.Unmarshal(entrance.Payload, &conf)
		if err != nil {
			return
		}
		aba.handleCONF(conf)

	case message.CoinType:
		var coin message.COIN
		err := json.Unmarshal(entrance.Payload, &coin)
		if err != nil {
			return
		}
		aba.handleCOIN(coin)
	}
}

// ABA event handler
func (aba *ABA) run() {
	defer func() {
		aba.logger.Printf("[Epoch:%d] [Lotteries:%d] ABA exit.\n", aba.epoch, aba.lotteries)
		aba.done <- struct{}{}
	}()

	// Wait for input value
	select {
	case <-aba.exit:
		aba.logger.Printf("[Epoch:%d] [ABAInstanceId:%d] capture exit signal.\n", aba.epoch, aba.lotteries)
		return
	case aba.est = <-aba.input:
	}

	aba.mu.Lock()
	if _, ok := aba.estSent[aba.round]; !ok {
		aba.estSent[aba.round] = make(map[int]bool)
	}

	ok := aba.estSent[aba.round][aba.est]
	aba.mu.Unlock()

	if ok {
		aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Peer:%d] [Round:%d] has sent est.\n",
			aba.epoch, aba.lotteries, aba.id, aba.round)
	} else {
		select {
		case <-aba.exit:
			return
		default:
			aba.sendEST(aba.round, aba.est)
		}
	}

	for {
		// Wait for binary values not null
	WaitBinary:
		for {
			select {
			case <-aba.exit:
				return
			case <-aba.event:
				aba.mu.Lock()
				if len(aba.binValues[aba.round]) != 0 {
					aba.mu.Unlock()
					break WaitBinary
				} else {
					aba.mu.Unlock()
				}
			}
		}

		aba.mu.Lock()
		b := aba.binValues[aba.round][len(aba.binValues[aba.round])-1]
		aba.mu.Unlock()
		aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] receive bin value = %d.\n",
			aba.epoch, aba.lotteries, aba.round, b)
		select {
		case <-aba.exit:
			return
		default:
			aba.sendAUX(aba.round, b)
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
		aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] AUX decide.\n",
			aba.epoch, aba.lotteries, aba.round)

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
		aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] CONF decide.\n",
			aba.epoch, aba.lotteries, aba.round)

		var coin int
		waitCh := make(chan struct{}, 1)

		go func() {
			for {
				select {
				case <-aba.exit:
					return
				case <-aba.event:
					aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] useless event.\n",
						aba.epoch, aba.lotteries, aba.round)
				case coin = <-aba.coinCh:
					select {
					case <-aba.exit:
						return
					case waitCh <- struct{}{}:
					}
					return
				}
			}
		}()

		// FIX unrelease aba instance bug
		select {
		case <-aba.exit:
			return
		case <-waitCh:
		}

		// aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] ABA values=%d coin=%d.\n",
		// 	aba.epoch, aba.lotteries, aba.round, values, coin)

		if stop := aba.setNewEst(values, coin); stop {
			return
		} else {
			aba.round++
			aba.logger.Printf("[Epoch:%d] [Lotteries:%d] ABA move to [Round:%d] with [EST:%d].\n",
				aba.epoch, aba.lotteries, aba.round, aba.est)
			select {
			case <-aba.exit:
				return
			default:
				aba.sendEST(aba.round, aba.est)
			}
		}
	}
}

// Set new estimate value
func (aba *ABA) setNewEst(values int, coin int) bool {
	aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] set New est values=%d coin=%d.\n",
		aba.epoch, aba.lotteries, aba.round, values, coin)
	stop := false
	if values != Both {
		if values == coin {
			if aba.alreadyDecide == nil {
				aba.alreadyDecide = &values
				select {
				case <-aba.exit:
					return stop
				default:
					abaOut := ABAOutput{decide: values}
					aba.epochEvent <- Event{eventType: DeliverABA, payload: abaOut}
				}
			} else if *aba.alreadyDecide == values {
				stop = true
			}
		}

		aba.est = values
	} else {
		aba.est = coin
	}

	return stop
}

// If receive >= 2f+1 valid aux msg, broadcast conf value
func (aba *ABA) auxThreshold(round int) bool {
	aba.mu.Lock()
	defer aba.mu.Unlock()

	// If receive >= 2f+1 aux msg with 1, broadcast (1.)
	if inSlice(1, aba.binValues[round]) && len(aba.auxValues[round][1]) >= aba.n-aba.f {
		aba.sendConf(round, 1)
		return true
	}

	// If receive >= 2f+1 aux msg with 0, broadcast (0.)
	if inSlice(0, aba.binValues[round]) && len(aba.auxValues[round][0]) >= aba.n-aba.f {
		aba.sendConf(round, 0)
		return true
	}

	// If receive >= 2f+1 aux msg with 0 & 1, broadcast (0,1)
	count := 0
	for _, v := range aba.binValues[round] {
		count += len(aba.auxValues[round][v])
	}
	if count >= aba.n-aba.f {
		aba.sendConf(round, Both)
		return true
	}
	return false
}

// If receive >= 2f+1 valid aux msg, broadcast coin share
func (aba *ABA) confThreshold(round int) (int, bool) {
	aba.mu.Lock()
	defer aba.mu.Unlock()

	// If receive >= 2f+1 conf msg with 1, set value to 1 for current round
	if inSlice(1, aba.binValues[round]) && len(aba.confValues[round][1]) >= aba.n-aba.f {
		aba.sendCoinShare(round)
		return 1, true
	}

	// If receive >= 2f+1 conf msg with 0, set value to 0 for curent round
	if inSlice(0, aba.binValues[round]) && len(aba.confValues[round][0]) >= aba.n-aba.f {
		aba.sendCoinShare(round)
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
		aba.sendCoinShare(round)
		return Both, true
	}

	return -1, false
}

// Upon receive est msg from other nodes, do this:
// If receive f+1 same est value, broadcast est
// If receive 2f+1 same est value, add to binary value
func (aba *ABA) handleEST(est message.EST) {
	aba.mu.Lock()

	if _, ok := aba.estValues[est.Round]; !ok {
		aba.estValues[est.Round] = make(map[int][]int)
	}

	if ok := inSlice(est.Sender, aba.estValues[est.Round][est.BinValue]); ok {
		aba.mu.Unlock()
		return
	}

	aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] [Peer:%d] receive [EST:%d] from [Sender:%d].\n",
		aba.epoch, aba.lotteries, est.Round, aba.id, est.BinValue, est.Sender)
	aba.estValues[est.Round][est.BinValue] = append(aba.estValues[est.Round][est.BinValue], est.Sender)
	estCount := len(aba.estValues[est.Round][est.BinValue])
	estSent := aba.estSent[est.Round][est.BinValue]

	aba.mu.Unlock()

	if estCount == aba.f+1 && !estSent {
		aba.sendEST(est.Round, est.BinValue)
	}

	if estCount == 2*aba.f+1 {
		aba.mu.Lock()
		if _, ok := aba.binValues[est.Round]; !ok {
			aba.binValues[est.Round] = make([]int, 0)
		}
		aba.binValues[est.Round] = append(aba.binValues[est.Round], est.BinValue)
		aba.mu.Unlock()
		aba.event <- struct{}{}
	}
}

func (aba *ABA) handleAUX(aux message.AUX) {
	aba.mu.Lock()
	if _, ok := aba.auxValues[aux.Round]; !ok {
		aba.auxValues[aux.Round] = make(map[int][]int)
	}

	if ok := inSlice(aux.Sender, aba.auxValues[aux.Round][aux.Element]); ok {
		aba.mu.Unlock()
		return
	}

	aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] [Peer:%d] receive [AUX:%d] from [Sender:%d].\n",
		aba.epoch, aba.lotteries, aux.Round, aba.id, aux.Element, aux.Sender)
	aba.auxValues[aux.Round][aux.Element] = append(aba.auxValues[aux.Round][aux.Element], aux.Sender)
	aba.mu.Unlock()
	aba.event <- struct{}{}
}

func (aba *ABA) handleCONF(conf message.CONF) {
	aba.mu.Lock()
	if _, ok := aba.confValues[conf.Round]; !ok {
		aba.confValues[conf.Round] = make(map[int][]int)
	}

	if ok := inSlice(conf.Sender, aba.confValues[conf.Round][conf.Value]); ok {
		aba.mu.Unlock()
		return
	}

	aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] [Peer:%d] receive [CONF:%d] from [Sender:%d].\n",
		aba.epoch, aba.lotteries, conf.Round, aba.id, conf.Value, conf.Sender)
	aba.confValues[conf.Round][conf.Value] = append(aba.confValues[conf.Round][conf.Value], conf.Sender)
	aba.mu.Unlock()

	aba.event <- struct{}{}
}

func (aba *ABA) handleCOIN(coin message.COIN) {
	aba.mu.Lock()
	if _, ok := aba.coinShares[coin.Round]; !ok {
		aba.coinShares[coin.Round] = make(map[int][]byte)
	}

	err := verify.VerifySignature(coin.HashMsg, coin.Signature, aba.pubKeys, coin.Sender)
	if err != nil {
		aba.mu.Unlock()
		return
	}

	aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] [Peer:%d] receive vaild Coin Share from [Sender:%d].\n",
		aba.epoch, aba.lotteries, coin.Round, aba.id, coin.Sender)

	if len(aba.coinShares[coin.Round]) > aba.f+1 {
		aba.mu.Unlock()
		return
	}

	if _, ok := aba.coinShares[coin.Round][coin.Sender]; ok {
		aba.mu.Unlock()
		return
	}

	aba.coinShares[coin.Round][coin.Sender] = coin.Signature
	if len(aba.coinShares[coin.Round]) == aba.f+1 {
		aba.mu.Unlock()
		aba.coinCh <- int(coin.HashMsg[0]) % 2
	} else {
		aba.mu.Unlock()
	}
}

func (aba *ABA) sendEST(round, est int) {
	aba.mu.Lock()
	if _, ok := aba.estSent[round]; !ok {
		aba.estSent[round] = make(map[int]bool)
	}
	aba.estSent[round][est] = true
	aba.mu.Unlock()

	aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] [Peer:%d] send [EST:%d].\n",
		aba.epoch, aba.lotteries, round, aba.id, est)

	abaEst := message.EST{
		Round:    round,
		Sender:   aba.id,
		BinValue: est,
	}
	abaEstJson, _ := json.Marshal(abaEst)

	abaEntrance := message.GenABAEntrance(message.EstType, aba.lotteries, abaEstJson)
	abaEntranceJson, _ := json.Marshal(abaEntrance)

	entrance := message.GenEntrance(message.ABAType, aba.epoch, abaEntranceJson)
	aba.broadcastEntrance(entrance)
}

func (aba *ABA) sendAUX(round, aux int) {
	aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] [Peer:%d] send [AUX:%d].\n",
		aba.epoch, aba.lotteries, round, aba.id, aux)
	abaAux := message.AUX{
		Round:   round,
		Sender:  aba.id,
		Element: aux,
	}
	abaAuxJson, _ := json.Marshal(abaAux)

	abaEntrance := message.GenABAEntrance(message.AuxType, aba.lotteries, abaAuxJson)
	abaEntranceJson, _ := json.Marshal(abaEntrance)

	entrance := message.GenEntrance(message.ABAType, aba.epoch, abaEntranceJson)
	aba.broadcastEntrance(entrance)
}

func (aba *ABA) sendConf(round, conf int) {
	aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] [Peer:%d] send [CONF:%d].\n",
		aba.epoch, aba.lotteries, round, aba.id, conf)
	abaConf := message.CONF{
		Round:  round,
		Sender: aba.id,
		Value:  conf,
	}
	abaConfJson, _ := json.Marshal(abaConf)

	abaEntrance := message.GenABAEntrance(message.ConfType, aba.lotteries, abaConfJson)
	abaEntranceJson, _ := json.Marshal(abaEntrance)

	entrance := message.GenEntrance(message.ABAType, aba.epoch, abaEntranceJson)
	aba.broadcastEntrance(entrance)
}

func (aba *ABA) sendCoinShare(round int) {
	str := strconv.Itoa(aba.lotteries) + "-" + strconv.Itoa(round)
	strJson, err := json.Marshal(str)
	if err != nil {
		return
	}

	strHash := sha256.Sum256(strJson)
	hashSignature, err := aba.priKey.Sign(strHash[:])
	if err != nil {
		return
	}

	aba.logger.Printf("[Epoch:%d] [Lotteries:%d] [Round:%d] [Peer:%d] broadcast coin share.\n",
		aba.epoch, aba.lotteries, round, aba.id)

	abaCoin := message.COIN{
		Round:     round,
		Sender:    aba.id,
		HashMsg:   strHash[:],
		Signature: hashSignature.Serialize(),
	}
	abaCoinJson, _ := json.Marshal(abaCoin)

	abaEntrance := message.GenABAEntrance(message.CoinType, aba.lotteries, abaCoinJson)
	abaEntranceJson, _ := json.Marshal(abaEntrance)

	entrance := message.GenEntrance(message.ABAType, aba.epoch, abaEntranceJson)
	aba.broadcastEntrance(entrance)
}

func (aba *ABA) broadcastEntrance(entrance message.Entrance) {
	select {
	case <-aba.stop:
		return
	default:
		aba.transport.Broadcast(entrance)
	}
}
