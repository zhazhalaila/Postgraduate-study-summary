package consensus

import (
	"errors"
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/zhazhalaila/PipelineBFT/src/genhash"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	"github.com/zhazhalaila/PipelineBFT/src/message"
	"github.com/zhazhalaila/PipelineBFT/src/verify"
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
	switch t := msg.(type) {
	case message.NewTransaction:
		pbc.handleNewTransaction(msg.(message.NewTransaction))

	case message.PrePrepare:
		pbc.handlePrePrepare(msg.(message.PrePrepare))

	case message.Prepare:
		pbc.handlePrepare(msg.(message.Prepare))

	default:
		pbc.logger.Println("Unkonwn Type: ", t)
	}
}

func (pbc *PBC) handleNewTransaction(newTxs message.NewTransaction) {
	pbc.logger.Printf("[Epoch%d] [Round:%d] [Proposer:%d] receive txs from client.\n",
		newTxs.Epoch, newTxs.Round, pbc.id)
	if newTxs.Initiator != pbc.id {
		return
	}

	// Generate hash for transactions
	txsHash, err := genhash.ConvertStructToHashBytes(newTxs.Transactions)
	if err != nil {
		pbc.logger.Printf("[Epoch:%d] [Round:%d] [Initiator:%d] generate hash for txs failed.\n",
			newTxs.Epoch, newTxs.Round, newTxs.Initiator)
		return
	}

	signature, err := pbc.priKey.Sign((*txsHash)[:])
	if err != nil {
		pbc.logger.Printf("[Epoch:%d] [Round:%d] [Initiator:%d] generate signature for txs failed.\n",
			newTxs.Epoch, newTxs.Round, newTxs.Initiator)
	}

	preprepareMsg := message.PrePrepare{
		Epoch:            newTxs.Epoch,
		Round:            newTxs.Round,
		Initiator:        newTxs.Initiator,
		TxsHash:          *txsHash,
		TxsHashSignature: signature.Serialize(),
		Transactions:     newTxs.Transactions,
	}

	pbc.transport.Broadcast(message.PreprepareType, preprepareMsg)

	pbc.logger.Printf("[Epoch:%d] [Round:%d] [Initiator:%d] broadcast new txs to all nodes.\n",
		newTxs.Epoch, newTxs.Round, pbc.id)
}

func (pbc *PBC) handlePrePrepare(prePrepare message.PrePrepare) {
	if prePrepare.Initiator != pbc.fromInitiator {
		pbc.logger.Printf("[Epoch:%d] [Round:%d] [Instance:%d] Get proposer = %d, want = %d.\n",
			prePrepare.Epoch, prePrepare.Round,
			pbc.fromInitiator, prePrepare.Initiator, pbc.fromInitiator)
		return
	}

	if _, ok := pbc.txs[prePrepare.TxsHash]; ok {
		pbc.logger.Printf("[Epoch:%d] [Round:%d] Receive redundant txs from [Initiator:%d].\n",
			prePrepare.Epoch, prePrepare.Round, prePrepare.Initiator)
		return
	}

	err := verify.VerifySignature(prePrepare.TxsHash[:], prePrepare.TxsHashSignature, pbc.pubKeys, prePrepare.Initiator)
	if err != nil {
		pbc.logger.Printf("[Epoch:%d] [Round:%d] Receive invalid preprepare msg from [Peer:%d].\n",
			prePrepare.Epoch, prePrepare.Round, prePrepare.Initiator)
		return
	}

	signature, err := pbc.priKey.Sign(prePrepare.TxsHash[:])
	if err != nil {
		pbc.logger.Println(err)
		return
	}

	prepareMsg := message.Prepare{
		Epoch:         prePrepare.Epoch,
		Round:         prePrepare.Round,
		Initiator:     prePrepare.Initiator,
		TxsHash:       prePrepare.TxsHash,
		Voter:         pbc.id,
		VoteSignature: signature.Serialize(),
	}

	pbc.txs[prePrepare.TxsHash] = prePrepare.Transactions
	pbc.transport.Broadcast(message.PrepareType, prepareMsg)

	pbc.logger.Printf("[Epoch:%d] [Round:%d] [Initiator:%d] [Peer:%d] broadcast vote to all nodes.\n",
		prePrepare.Epoch, prePrepare.Round, prePrepare.Initiator, pbc.id)
}

func (pbc *PBC) handlePrepare(prepare message.Prepare) {
	if _, ok := pbc.sigSets[prepare.TxsHash][prepare.Voter]; ok {
		pbc.logger.Printf("[Epoch:%d] [Round:%d] Receive redundant prepare msg from [Peer:%d].\n",
			prepare.Epoch, prepare.Round, prepare.Voter)
		return
	}

	err := verify.VerifySignature(prepare.TxsHash[:], prepare.VoteSignature, pbc.pubKeys, prepare.Voter)
	if err != nil {
		pbc.logger.Printf("[Epoch:%d] [Round:%d] Receive invalid prepare msg from [Peer:%d].\n",
			prepare.Epoch, prepare.Round, prepare.Voter)
		return
	}

	if _, ok := pbc.sigSets[prepare.TxsHash]; !ok {
		pbc.sigSets[prepare.TxsHash] = make(map[int][]byte)
	}
	pbc.sigSets[prepare.TxsHash][prepare.Voter] = prepare.VoteSignature

	for txsHash, collected := range pbc.sigSets {
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
			qc.Hash = txsHash[:]

			pbc.logger.Printf("[Epoch:%d] [Round:%d] generate QC for [Initiator:%d].\n",
				prepare.Epoch, prepare.Round, prepare.Initiator)

			// 		// send qc to epoch module
			// 		select {
			// 		case <-pbc.stop:
			// 			return
			// 		case pbc.epochEvent <- Event{
			// 			eventType:  PBCOUTPUT,
			// 			r:          pbc.r,
			// 			instanceId: pbc.fromInitiator,
			// 			qc:         qc,
			// 		}:
			// 		}
		}
	}
}
