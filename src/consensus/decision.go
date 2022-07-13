package consensus

import (
	"crypto/sha256"
	"encoding/json"
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	"github.com/zhazhalaila/PipelineBFT/src/message"
	"github.com/zhazhalaila/PipelineBFT/src/verify"
)

type DECISION struct {
	// Global log
	logger    *log.Logger
	transport *libnet.NetworkTransport

	n     int
	f     int
	id    int
	epoch int

	// collect all decision message
	collectDecisions map[int]map[int]message.Decision

	// public key
	pubKey *secp256k1.PublicKey
	// private key
	priKey *secp256k1.PrivateKey
	// public key for all peers
	pubKeys map[int]*secp256k1.PublicKey

	// done
	done chan struct{}
	// stop decision instance
	stop chan bool
	// read data from channel
	inCh chan message.Decision
	// return result to epoch module
	epochEvent chan Event
}

func MakeDecision(
	logger *log.Logger,
	transport *libnet.NetworkTransport,
	n, f, id, epoch int,
	pubKey *secp256k1.PublicKey,
	priKey *secp256k1.PrivateKey,
	pubKeys map[int]*secp256k1.PublicKey,
	epochEvent chan Event) *DECISION {
	d := &DECISION{}
	d.logger = logger
	d.transport = transport
	d.n = n
	d.f = f
	d.id = id
	d.collectDecisions = make(map[int]map[int]message.Decision)
	d.epoch = epoch
	d.pubKey = pubKey
	d.priKey = priKey
	d.pubKeys = pubKeys
	d.done = make(chan struct{})
	d.stop = make(chan bool)
	d.inCh = make(chan message.Decision, d.n)
	d.epochEvent = epochEvent
	go d.run()
	return d
}

func (d *DECISION) Input(msg message.Decision) {
	d.inCh <- msg
}

func (d *DECISION) Stop() {
	close(d.stop)
}

func (d *DECISION) run() {
	defer func() {
		d.done <- struct{}{}
	}()

	for {
		select {
		case <-d.stop:
			return
		case msg := <-d.inCh:
			d.handleDecision(msg)
		}
	}
}

func (d *DECISION) handleDecision(decision message.Decision) {
	if _, ok := d.collectDecisions[decision.Producer][decision.Sender]; ok {
		return
	}

	if _, ok := d.collectDecisions[decision.Producer]; !ok {
		d.collectDecisions[decision.Producer] = make(map[int]message.Decision)
	}

	d.logger.Printf("[Epoch:%d] [Peer:%d] receive decision msg from {Sender:%d, Producer:%d}.\n",
		d.epoch, d.id, decision.Sender, decision.Producer)

	d.collectDecisions[decision.Producer][decision.Sender] = decision

	for _, producerDecisions := range d.collectDecisions {
		if len(producerDecisions) == 2*d.f+1 {
			for _, recvDecision := range producerDecisions {
				if d.validate(recvDecision) {
					select {
					case <-d.stop:
						return
					default:
						decisionOut := DecisionOutput{
							producer: recvDecision.Producer,
							qcs:      &recvDecision.Qcs,
						}
						d.epochEvent <- Event{eventType: DeliverDecision, payload: decisionOut}
						return
					}
				}
			}

		}
	}
}

func (d *DECISION) validate(decision message.Decision) bool {
	if !decision.Delivered {
		return false
	}

	qcsJson, err := json.Marshal(decision.Qcs)
	if err != nil {
		return false
	}
	qcsHash := sha256.Sum256(qcsJson)

	for endorser, sig := range decision.Proof {
		err := verify.VerifySignature(qcsHash[:], sig, d.pubKeys, endorser)
		if err != nil {
			d.logger.Printf("[Epoch:%d] Receive invalid decision msg caused by [Endorser:%d].\n",
				d.epoch, endorser)
			return false
		}
	}

	return true
}
