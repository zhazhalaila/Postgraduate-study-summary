package consensus

import (
	"bytes"
	"encoding/json"
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	"github.com/zhazhalaila/PipelineBFT/src/message"
	"github.com/zhazhalaila/PipelineBFT/src/verify"
)

type CBC struct {
	// Global log
	logger    *log.Logger
	transport *libnet.NetworkTransport

	n             int
	f             int
	id            int
	epoch         int
	r             int
	fromCandidate int

	qcsHash []byte
	// collect all voters' signatures, only candidate do this
	sigSets map[int][]byte

	// public key
	pubKey *secp256k1.PublicKey
	// private key
	priKey *secp256k1.PrivateKey
	// public key for all peers
	pubKeys map[int]*secp256k1.PublicKey

	// delivered path
	selfPath *Path

	// done
	done chan struct{}
	// stop cbcc instance
	stop chan bool
	// read data from channel
	inCh chan message.CBCEntrance
	// return result to epoch module
	epochEvent chan Event
}

func MakeCBC(
	logger *log.Logger,
	transport *libnet.NetworkTransport,
	n, f, id, epoch, r, fromCandidate int,
	pubKey *secp256k1.PublicKey,
	priKey *secp256k1.PrivateKey,
	pubKeys map[int]*secp256k1.PublicKey,
	selfPath *Path,
	epochEvent chan Event) *CBC {
	cbc := &CBC{}

	cbc.logger = logger
	cbc.transport = transport
	cbc.n = n
	cbc.f = f
	cbc.id = id
	cbc.epoch = epoch
	cbc.r = r
	cbc.fromCandidate = fromCandidate
	cbc.sigSets = make(map[int][]byte)
	cbc.pubKey = pubKey
	cbc.priKey = priKey
	cbc.pubKeys = pubKeys
	cbc.selfPath = selfPath
	cbc.done = make(chan struct{})
	cbc.stop = make(chan bool)
	cbc.inCh = make(chan message.CBCEntrance, cbc.n*cbc.n)
	cbc.epochEvent = epochEvent
	go cbc.run()
	return cbc
}

func (cbc *CBC) Input(msg message.CBCEntrance) {
	cbc.inCh <- msg
}

func (cbc *CBC) Stop() {
	close(cbc.stop)
}

func (cbc *CBC) run() {
	defer func() {
		cbc.done <- struct{}{}
	}()

	for {
		select {
		case <-cbc.stop:
			return
		case entrance := <-cbc.inCh:
			cbc.handleMsg(entrance)
		}
	}
}

func (cbc *CBC) handleMsg(entrance message.CBCEntrance) {
	switch entrance.SpecificType {
	case message.CBCSendType:
		var send message.CBCSEND
		err := json.Unmarshal(entrance.Payload, &send)
		if err != nil {
			return
		}
		cbc.handleCBCSend(send)

	case message.CBCAckType:
		var ack message.CBCACK
		err := json.Unmarshal(entrance.Payload, &ack)
		if err != nil {
			return
		}
		cbc.handleCBCAck(ack)

	case message.CBCDoneType:
		var done message.CBCDONE
		err := json.Unmarshal(entrance.Payload, &done)
		if err != nil {
			return
		}
		cbc.handleCBCDone(done)

	default:
		cbc.logger.Println("Unkonwn Type")
	}
}

func (cbc *CBC) handleCBCSend(send message.CBCSEND) {
	// If Candidate is not excecept Candidate, return
	if send.Candidate != cbc.fromCandidate {
		cbc.logger.Printf("[Epoch:%d] [Round:%d] [Instance:%d] Get candidate = %d, want = %d.\n",
			cbc.epoch, cbc.r, cbc.fromCandidate, send.Candidate, cbc.fromCandidate)
		return
	}

	err := verify.VerifySignature(send.QcsHash, send.QcsHashSignature, cbc.pubKeys, send.Candidate)
	if err != nil {
		cbc.logger.Printf("[Epoch:%d] [Round:%d] Receive invalid CBC_SEND msg from [Candidate:%d].\n",
			cbc.epoch, cbc.r, cbc.fromCandidate)
		return
	}

	// External validate
	for _, roundQcs := range *send.Qcs {
		for _, qc := range roundQcs {
			if cbc.selfPath.Exist(qc) {
				continue
			}
			for voter, sig := range qc.Signatures {
				err := verify.VerifySignature(send.QcsHash, sig, cbc.pubKeys, voter)
				if err != nil {
					cbc.logger.Printf("[Epoch:%d] [Round:%d] Receive invalid CBC_SEND msg from [Candidate:%d] caused by [QC:%d].\n",
						cbc.epoch, cbc.r, cbc.fromCandidate, qc.Initiator)
					return
				}
			}
		}
	}

	voteSignature, err := cbc.priKey.Sign(send.QcsHash)
	if err != nil {
		cbc.logger.Println(err)
		return
	}

	cbc.logger.Printf("[Epoch:%d] [Round:%d] [Peer:%d] Receive valid CBC_SEND msg from [Candidate:%d].\n",
		cbc.epoch, cbc.r, cbc.id, send.Candidate)

	cbc.qcsHash = send.QcsHash

	ack := message.CBCACK{
		QcsHash:       cbc.qcsHash,
		VoteSignature: voteSignature.Serialize(),
		Voter:         cbc.id,
	}
	ackJson, _ := json.Marshal(ack)

	cbcEntrance := message.GenCBCEntrance(message.CBCAckType, cbc.r, cbc.fromCandidate, ackJson)
	cbcEntranceJson, _ := json.Marshal(cbcEntrance)

	entrance := message.GenEntrance(message.CBCType, cbc.epoch, cbcEntranceJson)

	// Send ACK to initiator
	select {
	case <-cbc.stop:
		return
	default:
		cbc.transport.SendToPeer(cbc.fromCandidate, entrance)
	}
}

func (cbc *CBC) handleCBCAck(ack message.CBCACK) {
	if !bytes.Equal(ack.QcsHash, cbc.qcsHash) {
		return
	}

	// If receive redundant ACK, return
	if _, ok := cbc.sigSets[ack.Voter]; ok {
		cbc.logger.Printf("[Epoch:%d] [Round:%d] Receive redundant CBC_ACK msg from [Voter:%d].\n",
			cbc.epoch, cbc.r, ack.Voter)
		return
	}

	err := verify.VerifySignature(ack.QcsHash, ack.VoteSignature, cbc.pubKeys, ack.Voter)
	if err != nil {
		cbc.logger.Printf("[Epoch:%d] [Round:%d] Receive invalid CBC_ACK msg from [Voter:%d].\n",
			cbc.epoch, cbc.r, ack.Voter)
		return
	}

	cbc.logger.Printf("[Epoch:%d] [Round:%d] [Candidate:%d] Receive valid CBC_ACK msg from [Voter:%d].\n",
		cbc.epoch, cbc.r, cbc.id, ack.Voter)

	cbc.sigSets[ack.Voter] = ack.VoteSignature

	// If acquire 2f+1 ACK, broadcast DONE
	if len(cbc.sigSets) == 2*cbc.f+1 {
		done := message.CBCDONE{
			Candidate: cbc.id,
			QcsHash:   cbc.qcsHash,
			Proof:     cbc.sigSets,
		}
		doneJson, _ := json.Marshal(done)

		pbEntrance := message.GenPBEntrance(message.CBCDoneType, cbc.r, cbc.id, doneJson)
		pbEntranceJson, _ := json.Marshal(pbEntrance)

		entrance := message.GenEntrance(message.CBCType, cbc.epoch, pbEntranceJson)

		select {
		case <-cbc.stop:
			return
		default:
			cbc.transport.Broadcast(entrance)
		}

		cbc.logger.Printf("[Epoch:%d] [Round:%d] [Candidate:%d] Broadcast Proof.\n",
			cbc.epoch, cbc.r, cbc.id)
	}
}

func (cbc *CBC) handleCBCDone(done message.CBCDONE) {
	if done.Candidate != cbc.fromCandidate {
		return
	}

	for voter, sig := range done.Proof {
		err := verify.VerifySignature(done.QcsHash, sig, cbc.pubKeys, voter)
		if err != nil {
			cbc.logger.Printf("[Epoch:%d] [Round:%d] Receive invalid CBC_DONE msg from [Candidate:%d] caused by [Voter:%d].\n",
				cbc.epoch, cbc.r, cbc.fromCandidate, voter)
			return
		}
	}

	cbc.logger.Printf("[Epoch:%d] [Round:%d] deliver CBC instance [Candidate:%d].\n", cbc.epoch, cbc.r, cbc.fromCandidate)
}
