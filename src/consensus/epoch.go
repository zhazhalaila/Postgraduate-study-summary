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
	DeliverLottery
	DeliverABA
	DeliverDecision
	DeliverRecovery
)

type Event struct {
	eventType uint8
	payload   interface{}
}

type PBOutput struct {
	rootHash [32]byte
	branch   *[][32]byte
	shard    *[]byte
	qc       message.QuorumCert
}

type CBCOutput struct {
	candidateId int
	qcsHash     [32]byte
	proof       map[int][]byte
	qcs         *[][]message.QuorumCert
}

type LotteryOutput struct {
	lotteryTimes   int
	commonProducer int
}

type ABAOutput struct {
	decide int
}

type DecisionOutput struct {
	producer int
	qcs      *[][]message.QuorumCert
}

type RecoveryOutput struct {
	batchSize int
}

type Epoch struct {
	// Global log
	logger    *log.Logger
	transport *libnet.NetworkTransport

	startTime   time.Time
	elapsedTime time.Duration

	n     int
	f     int
	id    int
	epoch int
	// maxRound reference how many rounds concurrently process in each epoch
	maxRound       int
	k              int
	lotteryTimes   int
	currProducer   int
	broadcastedQcs bool
	receiveAll     bool

	// public key
	pubKey *secp256k1.PublicKey
	// private key
	priKey *secp256k1.PrivateKey
	// public key for all peers
	pubKeys map[int]*secp256k1.PublicKey

	path    *Path
	cbcOuts map[int]CBCOutput

	pbs          map[int]map[int]*PB
	cbcs         map[int]*CBC
	lotteries    map[int]*Lottery
	abas         map[int]*ABA
	selfDecision *DECISION
	seltRecovery *RECOVERY

	producerQcs *[][]message.QuorumCert

	event chan Event
	stop  chan bool
	inCh  chan message.Entrance

	epochExitCh    chan int
	garbageCollect chan int
}

func MakeEpoch(
	logger *log.Logger,
	transport *libnet.NetworkTransport,
	n, f, id, epoch, maxRound int,
	pubKey *secp256k1.PublicKey,
	priKey *secp256k1.PrivateKey,
	pubKeys map[int]*secp256k1.PublicKey,
	epochExitCh, garbageCollect chan int) *Epoch {
	e := &Epoch{}
	e.logger = logger
	e.transport = transport
	e.startTime = time.Now()
	e.n = n
	e.f = f
	e.id = id
	e.epoch = epoch
	e.maxRound = maxRound
	e.k = 0
	e.lotteryTimes = 0
	e.broadcastedQcs = false
	e.receiveAll = false
	e.pubKey = pubKey
	e.priKey = priKey
	e.pubKeys = pubKeys
	e.path = NewPath(e.n, e.f, e.maxRound)
	e.cbcOuts = make(map[int]CBCOutput)
	e.pbs = make(map[int]map[int]*PB, e.maxRound)
	e.cbcs = make(map[int]*CBC, e.n)
	e.lotteries = make(map[int]*Lottery, e.n)
	e.abas = make(map[int]*ABA, e.n)
	e.event = make(chan Event, e.n*e.n*e.maxRound)
	e.stop = make(chan bool)
	e.inCh = make(chan message.Entrance, e.n*e.n*e.maxRound)
	e.epochExitCh = epochExitCh
	e.garbageCollect = garbageCollect

	// Initiate pb instances
	for i := 0; i < e.maxRound; i++ {
		// Create PB instance for each round
		e.pbs[i] = make(map[int]*PB, e.n)
		for j := 0; j < e.n; j++ {
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
			e.pubKey, e.priKey, e.pubKeys,
			e.event)
	}

	e.selfDecision = MakeDecision(
		e.logger, e.transport,
		e.n, e.f, e.id, e.epoch,
		e.pubKey, e.priKey, e.pubKeys,
		e.event)

	e.seltRecovery = MakeRecovery(
		e.logger, e.transport,
		e.n, e.f, e.id, e.epoch, e.maxRound,
		e.event)

	e.producerQcs = nil
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
	close(e.stop)
}

func (e *Epoch) run() {
L:
	for {
		select {
		case <-e.stop:
			break L
		case msg := <-e.inCh:
			e.handleMsg(msg)
		case event := <-e.event:
			e.handleEvent(event)
		}
	}

	// Stop all PB instance
	for i := 0; i < e.maxRound; i++ {
		for j := 0; j < e.n; j++ {
			e.pbs[i][j].Stop()
		}
	}

	e.logger.Printf("[Epoch:%d] stop all PB instance.\n", e.epoch)

	// Stop all CBC&&Lotery instance
	for i := 0; i < e.n; i++ {
		e.cbcs[i].Stop()
		e.lotteries[i].Stop()
	}

	// Stop activated ABA instance
	for i := 0; i < e.lotteryTimes; i++ {
		e.abas[i].Stop()
	}

	e.logger.Printf("[Epoch:%d] stop all CBC&&Lotery&&ABA instance.\n", e.epoch)

	// Stop Decision and Recovery instance
	e.selfDecision.Stop()
	e.seltRecovery.Stop()

	e.logger.Printf("[Epoch:%d] stop Decision and Recovery instance.\n", e.epoch)

	// Wait for all PB instances exit
	for i := 0; i < e.maxRound; i++ {
		for j := 0; j < e.n; j++ {
			<-e.pbs[i][j].Done()
		}
	}
	e.logger.Printf("[Epoch:%d] wait for all PB instances exit.\n", e.epoch)

	// Wait for all CBC&&Lotery&&ABA instance exit
	for i := 0; i < e.n; i++ {
		<-e.cbcs[i].Done()
		<-e.lotteries[i].Done()
	}

	for i := 0; i < e.lotteryTimes; i++ {
		<-e.abas[i].Done()
	}

	// Wait for Decision and Recovery instance exit
	<-e.selfDecision.Done()
	<-e.seltRecovery.Done()

	e.logger.Printf("[Epoch:%d] all sub instances done.\n", e.epoch)

	e.garbageCollect <- e.epoch
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
	case message.DecisionType:
		e.handleDecisionEntrance(req)
	case message.RecoveryType:
		e.handleRecoveryEntrance(req)
	}
}

func (e *Epoch) handlePBEntrance(req message.Entrance) {
	var pbEntrance message.PBEntrance
	err := json.Unmarshal(req.Payload, &pbEntrance)
	if err != nil {
		return
	}

	if pbEntrance.SpecificType == message.NewTransactionsType {
		if e.Full() {
			return
		}
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

func (e *Epoch) handleDecisionEntrance(req message.Entrance) {
	var decisionEntrance message.Decision
	err := json.Unmarshal(req.Payload, &decisionEntrance)
	if err != nil {
		return
	}

	e.selfDecision.Input(decisionEntrance)
}

func (e *Epoch) handleRecoveryEntrance(req message.Entrance) {
	var recoveryEntrance message.RECOVERY
	err := json.Unmarshal(req.Payload, &recoveryEntrance)
	if err != nil {
		return
	}

	e.seltRecovery.Input(recoveryEntrance)
}

func (e *Epoch) handleEvent(event Event) {
	switch event.eventType {
	case DeliverPB:
		e.handlePBOut(event.payload.(PBOutput))
	case DeliverCBC:
		e.handleCBCOut(event.payload.(CBCOutput))
	case DeliverLottery:
		e.handleLotteryOut(event.payload.(LotteryOutput))
	case DeliverABA:
		e.handleABAOut(event.payload.(ABAOutput))
	case DeliverDecision:
		e.handleDecisionOut(event.payload.(DecisionOutput))
	case DeliverRecovery:
		e.handleRecoveryOut(event.payload.(RecoveryOutput))
	}
}

func (e *Epoch) handlePBOut(pbOut PBOutput) {
	if e.path.Exist(pbOut.qc) {
		return
	}

	e.path.Add(pbOut.qc, pbOut.rootHash, pbOut.branch, pbOut.shard)

	e.logger.Printf("[Epoch:%d] [K:%d] [Qcs:%d].\n", e.epoch, e.k, e.path.Len())

	if !e.path.RecvThreshold() || e.broadcastedQcs {
		return
	}

	if e.producerQcs != nil && !e.receiveAll {
		e.broadcastRecovery()
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
	case <-e.stop:
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

	e.broadcastCoinShare()
}

func (e *Epoch) broadcastCoinShare() {
	e.logger.Printf("[Epoch:%d] [LotteryTimes:%d] [Peer:%d] broadcast Coin Share.\n",
		e.epoch, e.lotteryTimes, e.id)
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
	case <-e.stop:
		return
	default:
		e.transport.Broadcast(entrance)
	}
}

func (e *Epoch) handleLotteryOut(lotteryOut LotteryOutput) {
	e.currProducer = lotteryOut.commonProducer
	e.logger.Printf("[Epoch:%d] [LotteryTimes:%d] get common [Producer:%d].\n",
		e.epoch, lotteryOut.lotteryTimes, lotteryOut.commonProducer)
	e.logger.Printf("[Epoch:%d] [LotteryTimes:%d] start ABA.\n", e.epoch, lotteryOut.lotteryTimes)

	if _, ok := e.cbcOuts[lotteryOut.commonProducer]; ok {
		e.abas[lotteryOut.lotteryTimes].InputEST(1)
	} else {
		e.abas[lotteryOut.lotteryTimes].InputEST(0)
	}
}

func (e *Epoch) handleABAOut(abaOut ABAOutput) {
	e.logger.Printf("[Epoch:%d] [LotteryTimes:%d] ABA done with [Output:%d].\n",
		e.epoch, e.lotteryTimes, abaOut.decide)

	if abaOut.decide == 1 {
		e.broadcastDecision()
	} else {
		e.lotteryTimes++
		e.broadcastCoinShare()
	}
}

func (e *Epoch) broadcastDecision() {
	decision := message.Decision{}
	if producerOut, ok := e.cbcOuts[e.currProducer]; !ok {
		decision.Delivered = false
	} else {
		decision.Producer = e.currProducer
		decision.Sender = e.id
		decision.Delivered = true
		decision.Proof = producerOut.proof
		decision.Qcs = *producerOut.qcs
	}
	decisionJson, _ := json.Marshal(decision)

	entrance := message.GenEntrance(message.DecisionType, e.epoch, decisionJson)

	select {
	case <-e.stop:
		return
	default:
		e.transport.Broadcast(entrance)
	}
}

func (e *Epoch) handleDecisionOut(decisionOut DecisionOutput) {
	e.logger.Printf("[Epoch:%d] will commit [Producer:%d]'s proposal.\n", e.epoch, decisionOut.producer)
	e.producerQcs = decisionOut.qcs
	// If receive all Qcs, broadcast recovery
	if !e.checkRecvAll() {
		return
	}

	e.receiveAll = true
	e.broadcastRecovery()
}

func (e *Epoch) checkRecvAll() bool {
	for _, roundQcs := range *e.producerQcs {
		for _, qc := range roundQcs {
			if !e.path.Exist(qc) {
				return false
			}
		}
	}
	return true
}

func (e *Epoch) broadcastRecovery() {
	echos := make(map[int]map[int]message.ECHO, e.maxRound)
	for i := 0; i < e.maxRound; i++ {
		echos[i] = make(map[int]message.ECHO)
	}
	for _, roundQcs := range *e.producerQcs {
		for _, qc := range roundQcs {
			echo := e.path.GetEcho(qc.Round, qc.Initiator)
			echos[qc.Round][qc.Initiator] = echo
		}
	}

	recoveryEntrance := message.GenRecovery(e.currProducer, e.id, echos)
	recoveryEntranceJson, _ := json.Marshal(recoveryEntrance)

	entrance := message.GenEntrance(message.RecoveryType, e.epoch, recoveryEntranceJson)

	e.logger.Printf("[Epoch:%d] [Peer:%d] broadcast Recovery.\n", e.epoch, e.id)
	select {
	case <-e.stop:
		return
	default:
		e.transport.Broadcast(entrance)

	}
}

func (e *Epoch) handleRecoveryOut(recoveryOut RecoveryOutput) {
	e.elapsedTime = time.Since(e.startTime)
	e.logger.Printf("\n [Epoch:%d] [K:%d] [Batchsize:%d] within [%d] millseconds.\n",
		e.epoch, e.maxRound, recoveryOut.batchSize, e.elapsedTime.Milliseconds())

	select {
	case <-e.stop:
		return
	default:
		e.epochExitCh <- e.epoch
		e.logger.Printf("[Epoch:%d] [Peer:%d] notify consensus epoch done.\n", e.epoch, e.id)
	}
}
