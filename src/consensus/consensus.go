package consensus

import (
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/zhazhalaila/PipelineBFT/src/keys"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	"github.com/zhazhalaila/PipelineBFT/src/message"
)

type ConsensusModule struct {
	logger    *log.Logger
	transport *libnet.NetworkTransport
	n         int
	f         int
	id        int
	epoch     int
	maxRound  int
	// public key
	pubKey *secp256k1.PublicKey
	// private key
	priKey *secp256k1.PrivateKey
	// public key for all peers
	pubKeys map[int]*secp256k1.PublicKey
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
	cm.epoch = 0
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
		case <-cm.transport.Stopped():
			break L
		}
	}
	cm.logger.Println("Consensus module break")
}

func (cm *ConsensusModule) handleMsg(req libnet.Request) {
	switch req.Msg.(type) {
	case message.PrePrepare:
		log.Println(req.Msg.(message.PrePrepare).Initiator)
	}
}
