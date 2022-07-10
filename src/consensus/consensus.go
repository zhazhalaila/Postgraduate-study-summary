package consensus

import (
	"encoding/json"
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

func (cm *ConsensusModule) handleMsg(req message.Entrance) {
	// cm.logger.Println("Consensus: ", req)

	// If epoch not create, create it.
	if req.ModuleType == message.PBType {
		var pbEntrance message.PBEntrance
		err := json.Unmarshal(req.Payload, &pbEntrance)
		if err != nil {
			return
		}

		if pbEntrance.SpecificType == message.NewTransactionsType {
			if epoch, ok := cm.epochs[cm.epoch]; ok && !epoch.Full() {
				epoch.Input(req)
			} else if !ok {
				cm.inputEpoch(cm.epoch, req)
			} else if epoch.Full() {
				cm.epoch++
				cm.inputEpoch(cm.epoch, req)
			}
			return
		}
	}

	// If epoch done, skip
	if _, ok := cm.epochDone[req.Epoch]; ok {
		return
	}

	if epoch, ok := cm.epochs[req.Epoch]; ok {
		epoch.Input(req)
	} else {
		cm.inputEpoch(req.Epoch, req)
	}
}

func (cm *ConsensusModule) makeNewEpoch(epoch int) *Epoch {
	e := MakeEpoch(
		cm.logger, cm.transport,
		cm.n, cm.f, cm.id, epoch, cm.maxRound,
		cm.pubKey, cm.priKey, cm.pubKeys)
	return e
}

func (cm *ConsensusModule) inputEpoch(epoch int, req message.Entrance) {
	cm.epochs[epoch] = cm.makeNewEpoch(epoch)
	cm.epochs[epoch].Input(req)
}
