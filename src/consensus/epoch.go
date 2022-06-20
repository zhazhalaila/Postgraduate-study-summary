package consensus

import (
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	"github.com/zhazhalaila/PipelineBFT/src/message"
)

const (
	PBCOUTPUT = iota
	PRODUCER  = iota
)

type EpochReq struct {
	round     int
	initiator int
	msg       interface{}
}

type Event struct {
	eventType  int
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
	// k reference how many rounds concurrently process in each epoch
	maxRound int

	// public key
	pubKey *secp256k1.PublicKey
	// private key
	priKey *secp256k1.PrivateKey
	// public key for all peers
	pubKeys map[int]*secp256k1.PublicKey

	path *Path
	pbcs map[int]map[int]*PBC

	event  chan Event
	stopCh chan bool
	inCh   chan EpochReq
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
	e.pubKey = pubKey
	e.priKey = priKey
	e.pubKeys = pubKeys
	e.path = NewPath(e.n, e.f, e.maxRound)
	e.pbcs = make(map[int]map[int]*PBC, e.n)
	e.event = make(chan Event, e.n*e.maxRound)
	e.inCh = make(chan EpochReq, e.n*e.n*e.maxRound)

	// Initiate pbc instances
	for i := 0; i < maxRound; i++ {
		e.pbcs[i] = make(map[int]*PBC, e.n)
		for j := 0; j < n; j++ {
			e.pbcs[i][j] = MakePBC(
				e.logger, e.transport, e.n, e.f, e.id, e.epoch, j, j, e.pubKey, e.priKey, e.pubKeys, e.event)
		}
	}

	go e.run()
	return e
}

func (e *Epoch) Input(msg EpochReq) {
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

func (e *Epoch) handleMsg(epochReq EpochReq) {
	switch epochReq.msg.(type) {
	case message.PrePrepare, message.Prepare:
		e.pbcs[epochReq.round][epochReq.initiator].Input(epochReq.msg)
	}
}

func (e *Epoch) handleEvent(event Event) {
	switch event.eventType {
	case PBCOUTPUT:
		e.handlePBCOut(event.r, event.qc)
	}
}

func (e *Epoch) handlePBCOut(r int, qc message.QuorumCert) {
	if e.path.Exist(r, qc) {
		return
	}

	e.path.Add(r, qc)
	if e.path.RecvThreshold() {
		// broadcast to all
	}
}
