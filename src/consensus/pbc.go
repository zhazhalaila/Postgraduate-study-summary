package consensus

import (
	"errors"
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	"github.com/zhazhalaila/PipelineBFT/src/message"
)

var (
	ErrPeerNotFound = errors.New("peer not found")
	ErrVerifyFail   = errors.New("signature verify fail")
)

type PBC struct {
	// Global log
	logger    *log.Logger
	transport *libnet.NetworkTransport

	n                 int
	f                 int
	id                int
	epoch             int
	r                 int
	fromInitiator     int
	preparedThreshold int

	// using map to cache txs prevent Byzantine leader
	txs map[[32]byte]*[][]byte
	// collect all voters' signatures
	sigSets map[[32]byte]map[int][]byte

	// public key
	pubKey *secp256k1.PublicKey
	// private key
	priKey *secp256k1.PrivateKey
	// public key for all peers
	pubKeys map[int]*secp256k1.PublicKey

	// done
	done chan struct{}
	// stop pbc instance
	stop chan bool
	// read data from channel
	inCh chan interface{}
	// return result to epoch module
	epochEvent chan Event
}

func MakePBC(logger *log.Logger,
	transport *libnet.NetworkTransport,
	n, f, id, epoch, r, fromInitiator int,
	pubKey *secp256k1.PublicKey,
	priKey *secp256k1.PrivateKey,
	pubKeys map[int]*secp256k1.PublicKey,
	epochEvent chan Event) *PBC {
	pbc := &PBC{}
	pbc.logger = logger
	pbc.transport = transport
	pbc.n = n
	pbc.f = f
	pbc.id = id
	pbc.epoch = epoch
	pbc.r = r
	pbc.fromInitiator = fromInitiator
	pbc.preparedThreshold = 2*pbc.f + 1
	pbc.txs = make(map[[32]byte]*[][]byte)
	pbc.sigSets = make(map[[32]byte]map[int][]byte)
	pbc.pubKey = pubKey
	pbc.priKey = priKey
	pbc.pubKeys = pubKeys
	pbc.done = make(chan struct{})
	pbc.stop = make(chan bool)
	pbc.inCh = make(chan interface{}, pbc.n*pbc.n)
	pbc.epochEvent = epochEvent
	go pbc.run()
	return pbc
}

func (pbc *PBC) Input(msg interface{}) {
	pbc.inCh <- msg
}

func (pbc *PBC) Stop() {
	close(pbc.stop)
}

func (pbc *PBC) run() {
	defer func() {
		pbc.done <- struct{}{}
	}()

	for {
		select {
		case <-pbc.stop:
			return
		case msg := <-pbc.inCh:
			pbc.handleCommand(msg)
		}
	}
}

func (pbc *PBC) handleCommand(msg interface{}) {
	switch msg.(type) {
	case message.PrePrepare:
		pbc.handlePrePrepare(msg.(message.PrePrepare))
	case message.Prepare:
		pbc.handlePrepare(msg.(message.Prepare))
	}
}

func (pbc *PBC) verifySignature(msgHash []byte, msgHashSignature []byte, sender int) error {
	signature, err := secp256k1.ParseDERSignature(msgHashSignature)
	if err != nil {
		pbc.logger.Printf("Parse signature from [Peer:%d] error.\n", sender)
		return err
	}

	initiatorPubKey, ok := pbc.pubKeys[sender]
	if !ok {
		pbc.logger.Printf("Not record [%d] pubkey.\n", sender)
		return ErrPeerNotFound
	}

	if verified := signature.Verify(msgHash, initiatorPubKey); !verified {
		pbc.logger.Printf("Receive invalid signature from [Peer:%d].\n", sender)
		return ErrVerifyFail
	}
	return nil
}

func (pbc *PBC) handlePrePrepare(prePrepare message.PrePrepare) {
	if prePrepare.Initiator != pbc.fromInitiator {
		pbc.logger.Printf("[Instance:%d] Get proposer = %d, want = %d.\n", pbc.fromInitiator,
			prePrepare.Initiator, pbc.fromInitiator)
		return
	}

	if _, ok := pbc.txs[prePrepare.MerkleRoot]; ok {
		pbc.logger.Printf("Receive redundant txs from [Initiator:%d].\n", prePrepare.Initiator)
		return
	}

	err := pbc.verifySignature(prePrepare.MerkleRoot[:], prePrepare.MerkleRootSignature, prePrepare.Initiator)
	if err != nil {
		pbc.logger.Printf("Receive invalid preprepare msg from [Peer:%d].\n", prePrepare.Initiator)
		return
	}

	signature, err := pbc.priKey.Sign(prePrepare.MerkleRoot[:])
	if err != nil {
		pbc.logger.Println(err)
		return
	}

	prepareMsg := message.Prepare{
		Epoch:         prePrepare.Epoch,
		Count:         prePrepare.Count,
		MerkleRoot:    prePrepare.MerkleRoot,
		Voter:         pbc.id,
		VoteSignature: signature.Serialize(),
	}

	pbc.txs[prePrepare.MerkleRoot] = prePrepare.Transactions

	pbc.transport.Broadcast(prepareMsg)
}

func (pbc *PBC) handlePrepare(prepare message.Prepare) {
	if _, ok := pbc.sigSets[prepare.MerkleRoot][prepare.Voter]; ok {
		pbc.logger.Printf("Receive redundant prepare msg from [Peer:%d].\n", prepare.Voter)
		return
	}

	err := pbc.verifySignature(prepare.MerkleRoot[:], prepare.VoteSignature, prepare.Voter)
	if err != nil {
		pbc.logger.Printf("Receive invalid prepare msg from [Peer:%d].\n", prepare.Voter)
		return
	}

	if _, ok := pbc.sigSets[prepare.MerkleRoot]; !ok {
		pbc.sigSets[prepare.MerkleRoot] = make(map[int][]byte)
	}
	pbc.sigSets[prepare.MerkleRoot][prepare.Voter] = prepare.VoteSignature

	for hash, collected := range pbc.sigSets {
		if len(collected) == 2*pbc.f+1 {
			proofs := make([][]byte, 2*pbc.f+1)
			for _, sig := range collected {
				proofs = append(proofs, sig)
			}

			// generate qc
			qc := message.QuorumCert{}
			qc.Signatures = proofs
			qc.Proposer = pbc.fromInitiator
			qc.Epoch = pbc.epoch
			qc.R = pbc.r
			qc.Hash = hash[:]

			// send qc to epoch module
			select {
			case <-pbc.stop:
				return
			case pbc.epochEvent <- Event{
				eventType:  PBCOUTPUT,
				r:          pbc.r,
				instanceId: pbc.fromInitiator,
				qc:         qc,
			}:
			}
		} else {
			return
		}
	}
}
