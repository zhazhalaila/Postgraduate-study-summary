package consensus

import (
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/zhazhalaila/PipelineBFT/src/keys"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
)

type ConsensusModule struct {
	logger    *log.Logger
	transport *libnet.NetworkTransport
	n         int
	f         int
	id        int
	maxRound  int
	// public key
	pubKey *secp256k1.PublicKey
	// private key
	priKey *secp256k1.PrivateKey
	// public key for all peers
	pubKeys map[int]*secp256k1.PublicKey

	epochs    map[int]*Epoch
	epochDone map[int]bool
	epochCh   chan int
}

func MakeConsensusModule(logger *log.Logger,
	transport *libnet.NetworkTransport,
	n, f, id, maxRound int) *ConsensusModule {
	cm := &ConsensusModule{}
	cm.logger = logger
	cm.transport = transport
	cm.n = n
	cm.f = f
	cm.id = id
	cm.maxRound = maxRound

	// Read key pair for current peer
	priKey, pubKey, err := keys.DecodeKeyPair(cm.id)
	if err != nil {
		log.Fatal(err)
	}
	cm.priKey = priKey
	cm.pubKey = pubKey

	// Read all public keys
	pubKeys, err := keys.DecodePublicKeys(cm.n)
	if err != nil {
		log.Fatal(err)
	}
	cm.pubKeys = pubKeys

	cm.epochs = make(map[int]*Epoch)
	cm.epochDone = make(map[int]bool)
	// max epoch in concurrency
	cm.epochCh = make(chan int, 100)
	return cm
}

func (cm *ConsensusModule) Run() {
	go cm.readDataFromTransport()
}

func (cm *ConsensusModule) readDataFromTransport() {
L:
	for {
		select {
		case req := <-cm.transport.Consume():
			log.Println("Consensus: ", req)
			cm.handleMsg(req)

		case epoch := <-cm.epochCh:
			if _, ok := cm.epochDone[epoch]; !ok {
				cm.epochDone[epoch] = true
				cm.epochs[epoch] = nil
			}

		case <-cm.transport.Stopped():
			break L
		}
	}
	cm.logger.Println("Consensus module break")
}

func (cm *ConsensusModule) handleMsg(req libnet.Request) {
	cm.logger.Println(req)

	// If epoch done, skip
	if _, ok := cm.epochDone[req.Epoch]; ok {
		return
	}

	epochReq := EpochReq{
		round:     req.Round,
		initiator: req.Initiator,
		msg:       req.Msg,
	}

	if epoch, ok := cm.epochs[req.Epoch]; ok {
		epoch.Input(epochReq)
	} else {
		cm.epochs[req.Epoch] = MakeEpoch(
			cm.logger, cm.transport,
			cm.n, cm.f, cm.id, req.Epoch, cm.maxRound,
			cm.pubKey, cm.priKey, cm.pubKeys)
	}
}
