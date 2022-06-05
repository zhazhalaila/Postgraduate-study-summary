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

	// read data from channel
	inCh chan message.PBCWrapper
}

func MakePBC(logger *log.Logger,
	transport *libnet.NetworkTransport,
	n, f, id, fromInitiator int,
	pubKey *secp256k1.PublicKey,
	priKey *secp256k1.PrivateKey,
	pubKeys map[int]*secp256k1.PublicKey) *PBC {
	pbc := &PBC{}
	pbc.logger = logger
	pbc.transport = transport
	pbc.n = n
	pbc.f = f
	pbc.id = id
	pbc.fromInitiator = fromInitiator
	pbc.preparedThreshold = 2*pbc.f + 1
	pbc.txs = make(map[[32]byte]*[][]byte)
	pbc.sigSets = make(map[[32]byte]map[int][]byte)
	pbc.pubKey = pubKey
	pbc.priKey = priKey
	pbc.pubKeys = pubKeys
	pbc.inCh = make(chan message.PBCWrapper)
	go pbc.run()
	return pbc
}

func (pbc *PBC) run() {
	for {
		select {
		case msg := <-pbc.inCh:
			pbc.handleCommand(msg)
		}
	}
}

func (pbc *PBC) handleCommand(msg message.PBCWrapper) {
	switch msg.Msg.(type) {
	case message.PrePrepare:
		pbc.handlePrePrepare(msg.Msg.(message.PrePrepare))
	case message.Prepare:
		pbc.handlePrepare(msg.Msg.(message.Prepare))
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

	prepareMsg := message.Prepare{
		Epoch:      prePrepare.Epoch,
		Count:      prePrepare.Count,
		MerkleRoot: prePrepare.MerkleRoot,
		Voter:      pbc.id,
	}

	signature, err := pbc.priKey.Sign(prePrepare.MerkleRoot[:])
	if err != nil {
		pbc.logger.Println(err)
		return
	}

	prepareMsg.VoteSignature = signature.Serialize()

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

	for _, collected := range pbc.sigSets {
		if len(collected) == 2*pbc.f+1 {
			// notify epoch module
		} else {
			return
		}
	}
}
