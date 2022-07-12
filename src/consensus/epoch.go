package consensus

import (
	"crypto/sha256"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	"github.com/zhazhalaila/PipelineBFT/src/message"
)

const (
	DeliverPB uint8 = iota
	DeliverCBC
	PRODUCER
)

type Event struct {
	eventType uint8
	payload   interface{}
}

type PBOutput struct {
	qc message.QuorumCert
}

type CBCOutput struct {
	candidateId int
	qcsHash     [32]byte
	proof       map[int][]byte
	qcs         *[][]message.QuorumCert
}

type Epoch struct {
	// Global log
	logger    *log.Logger
	transport *libnet.NetworkTransport

	n     int
	f     int
	id    int
	epoch int
	// maxRound reference how many rounds concurrently process in each epoch
	maxRound     int
	k            int
	lotteryTimes int

	broadcastedQcs bool

	// public key
	pubKey *secp256k1.PublicKey
	// private key
	priKey *secp256k1.PrivateKey
	// public key for all peers
	pubKeys map[int]*secp256k1.PublicKey

	path    *Path
	cbcOuts map[int]CBCOutput

	pbs       map[int]map[int]*PB
	cbcs      map[int]*CBC
	lotteries map[int]*Lottery
	abas      map[int]*ABA

	event  chan Event
	stopCh chan bool
	inCh   chan message.Entrance
}

func MakeEpoch(
	logger *log.Logger,
	transport *libnet.NetworkTransport,
	n, f, id, epoch, maxRound int,
	pubKey *secp256k1.PublicKey,
	priKey *secp256k1.PrivateKey,
	pubKeys map[int]*secp256k1.PublicKey,
) *Epoch {
	e := &Epoch{}
	e.logger = logger
	e.transport = transport
	e.n = n
	e.f = f
	e.id = id
	e.epoch = epoch
	e.maxRound = maxRound
	e.k = 0
	e.lotteryTimes = 0
	e.broadcastedQcs = false
	e.pubKey = pubKey
	e.priKey = priKey
	e.pubKeys = pubKeys
	e.path = NewPath(e.n, e.f, e.maxRound)
	e.cbcOuts = make(map[int]CBCOutput)
	e.pbs = make(map[int]map[int]*PB, e.maxRound)
	e.cbcs = make(map[int]*CBC, e.n)
	e.lotteries = make(map[int]*Lottery, e.n)
	e.abas = make(map[int]*ABA, e.n)
	e.event = make(chan Event, e.n*e.maxRound)
	e.inCh = make(chan message.Entrance, e.n*e.n*e.maxRound)

	// Initiate pb instances
	for i := 0; i < maxRound; i++ {
		// Create PB instance for each round
		e.pbs[i] = make(map[int]*PB, e.n)
		for j := 0; j < n; j++ {
			e.pbs[i][j] = MakePB(
				e.logger, e.transport,
				e.n, e.f, e.id, e.epoch, i, j,
				e.pubKey, e.priKey, e.pubKeys, e.event)
		}
	}

	for i := 0; i < e.n; i++ {
		// Create CBC instance
		e.cbcs[i] = MakeCBC(
			e.logger, e.transport,
			e.n, e.f, e.id, e.epoch, i,
			e.pubKey, e.priKey, e.pubKeys,
			e.path,
			e.event)
		// Create Lottery instance
		e.lotteries[i] = MakeLottery(
			e.logger, e.transport,
			e.n, e.f, e.id, e.epoch, i,
			e.pubKey, e.priKey, e.pubKeys,
			e.event)
		// Create ABA instance
		e.abas[i] = MakeABA(
			e.logger, e.transport,
			e.n, e.f, e.id, e.epoch, i,
			e.pubKey, e.priKey, e.pubKeys)
	}

	go e.run()
	return e
}

func (e *Epoch) Full() bool {
	return e.k == e.maxRound
}

func (e *Epoch) Input(msg message.Entrance) {
	e.inCh <- msg
}

func (e *Epoch) Stop() {
	close(e.stopCh)
}

func (e *Epoch) run() {
L:
	for {
		select {
		case <-e.stopCh:
			break L
		case msg := <-e.inCh:
			e.handleMsg(msg)
		case event := <-e.event:
			e.handleEvent(event)
		}
	}
}

func (e *Epoch) handleMsg(req message.Entrance) {
	switch req.ModuleType {
	case message.PBType:
		e.handlePBEntrance(req)
	case message.CBCType:
		e.handleCBCEntrance(req)
	case message.LotteryType:
		e.handleLotteryEntrance(req)
	case message.ABAType:
		e.handleABAEntrance(req)
	}
}

func (e *Epoch) handlePBEntrance(req message.Entrance) {
	var pbEntrance message.PBEntrance
	err := json.Unmarshal(req.Payload, &pbEntrance)
	if err != nil {
		return
	}

	if pbEntrance.SpecificType == message.NewTransactionsType {
		e.pbs[e.k][e.id].Input(pbEntrance)
		e.k++
		e.logger.Printf("K=%d \n", e.k)
	} else {
		e.pbs[pbEntrance.Round][pbEntrance.Initiator].Input(pbEntrance)
	}
}

func (e *Epoch) handleCBCEntrance(req message.Entrance) {
	var cbcEntrance message.CBCEntrance
	err := json.Unmarshal(req.Payload, &cbcEntrance)
	if err != nil {
		return
	}

	e.cbcs[cbcEntrance.Candidate].Input(cbcEntrance)
}

func (e *Epoch) handleLotteryEntrance(req message.Entrance) {
	var lotteryEntrance message.Lottery
	err := json.Unmarshal(req.Payload, &lotteryEntrance)
	if err != nil {
		return
	}

	e.lotteries[lotteryEntrance.LotteryTimes].Input(lotteryEntrance)
}

func (e *Epoch) handleABAEntrance(req message.Entrance) {
	var abaEntrance message.ABAEntrance
	err := json.Unmarshal(req.Payload, &abaEntrance)
	if err != nil {
		return
	}

	e.abas[abaEntrance.LotteryTimes].InputValue(abaEntrance)
}

func (e *Epoch) handleEvent(event Event) {
	switch event.eventType {
	case DeliverPB:
		e.handlePBOut(event.payload.(PBOutput))
	case DeliverCBC:
		e.handleCBCOut(event.payload.(CBCOutput))
	}
}

func (e *Epoch) handlePBOut(pbOut PBOutput) {
	if e.path.Exist(pbOut.qc) {
		return
	}

	e.path.Add(pbOut.qc)

	if !e.path.RecvThreshold() || e.broadcastedQcs {
		return
	}

	e.broadcastPath()
}

func (e *Epoch) broadcastPath() {
	e.logger.Printf("[Epoch:%d] [Candidate:%d] broadcast Path.\n", e.epoch, e.id)
	qcs := e.path.GetQC()
	qcsJson, err := json.Marshal(qcs)
	if err != nil {
		return
	}
	qcsHash := sha256.Sum256(qcsJson)
	e.cbcs[e.id].AssignQcsHash(qcsHash, qcs)

	qcsSignature, err := e.priKey.Sign(qcsHash[:])
	if err != nil {
		e.logger.Println(err)
		return
	}

	cbcSend := message.CBCSEND{
		Candidate:        e.id,
		Qcs:              qcs,
		QcsHash:          qcsHash,
		QcsHashSignature: qcsSignature.Serialize(),
	}
	cbcJson, _ := json.Marshal(cbcSend)

	cbcEntrance := message.GenCBCEntrance(message.CBCSendType, e.id, cbcJson)
	cbcEntranceJson, _ := json.Marshal(cbcEntrance)

	entrance := message.GenEntrance(message.CBCType, e.epoch, cbcEntranceJson)

	select {
	case <-e.stopCh:
		return
	default:
		e.transport.Broadcast(entrance)
	}

	e.broadcastedQcs = true
}

func (e *Epoch) handleCBCOut(cbcOut CBCOutput) {
	e.logger.Printf("[Epoch:%d] handle [CBCInstance:%d] out.\n", e.epoch, cbcOut.candidateId)
	if _, ok := e.cbcOuts[cbcOut.candidateId]; ok {
		return
	}

	e.cbcOuts[cbcOut.candidateId] = cbcOut

	if len(e.cbcOuts) != 2*e.f+1 {
		return
	}

	go func() {
		time.Sleep(100 * time.Millisecond)

		for i := 0; i < 10; i++ {
			e.logger.Printf("\n")
		}

		e.logger.Printf("[Epoch:%d] [LotteryTimes:%d] start ABA.\n", e.epoch, e.lotteryTimes)

		if e.id%2 == 0 {
			e.abas[e.lotteryTimes].InputEST(1)
		} else {
			e.abas[e.lotteryTimes].InputEST(0)
		}
	}()

	// e.broadcastCoinShare()
}

func (e *Epoch) broadcastCoinShare() {
	e.logger.Printf("[Epoch:%d] [Peer:%d] broadcast Coin Share.\n", e.epoch, e.id)
	str := strconv.Itoa(e.epoch) + "-" + strconv.Itoa(e.lotteryTimes)
	strJson, err := json.Marshal(str)
	if err != nil {
		return
	}

	strHash := sha256.Sum256(strJson)
	lotterySignature, err := e.priKey.Sign(strHash[:])
	if err != nil {
		e.logger.Println(err)
		return
	}

	lottery := message.GenLottery(e.lotteryTimes, e.id, strHash, lotterySignature.Serialize())
	lotteryJson, _ := json.Marshal(lottery)

	entrance := message.GenEntrance(message.LotteryType, e.epoch, lotteryJson)

	select {
	case <-e.stopCh:
		return
	default:
		e.transport.Broadcast(entrance)
	}
}
