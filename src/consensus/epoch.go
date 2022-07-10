package consensus

import (
	"encoding/json"
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	"github.com/zhazhalaila/PipelineBFT/src/message"
)

const (
	DeliverPB uint8 = iota
	PRODUCER
)

type Event struct {
	eventType  uint8
	r          int
	instanceId int
	qc         message.QuorumCert
	producer   int
	decision   int
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
	maxRound int
	k        int

	// public key
	pubKey *secp256k1.PublicKey
	// private key
	priKey *secp256k1.PrivateKey
	// public key for all peers
	pubKeys map[int]*secp256k1.PublicKey

	path *Path
	pbs  map[int]map[int]*PB

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
	e.pubKey = pubKey
	e.priKey = priKey
	e.pubKeys = pubKeys
	e.path = NewPath(e.n, e.f, e.maxRound)
	e.pbs = make(map[int]map[int]*PB, e.maxRound)
	e.event = make(chan Event, e.n*e.maxRound)
	e.inCh = make(chan message.Entrance, e.n*e.n*e.maxRound)

	// Initiate pb instances
	for i := 0; i < maxRound; i++ {
		e.pbs[i] = make(map[int]*PB, e.n)
		for j := 0; j < n; j++ {
			e.pbs[i][j] = MakePB(
				e.logger, e.transport,
				e.n, e.f, e.id, e.epoch, i, j,
				e.pubKey, e.priKey, e.pubKeys, e.event)
		}
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
	}
}

func (e *Epoch) handlePBEntrance(req message.Entrance) {
	// e.logger.Println("Epoch: ", string(req.Payload))

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

func (e *Epoch) handleEvent(event Event) {
	switch event.eventType {
	case DeliverPB:
		e.handlePBCOut(event.qc)
	}
}

func (e *Epoch) handlePBCOut(qc message.QuorumCert) {
	if e.path.Exist(qc) {
		return
	}

	e.path.Add(qc)
	if e.path.RecvThreshold() {
		e.logger.Println("Deliver enough PB instance")
	}
}
