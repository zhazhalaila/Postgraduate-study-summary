package consensus

import (
	"log"

	"github.com/zhazhalaila/PipelineBFT/src/libnet"
)

type ConsensusModule struct {
	logger     *log.Logger
	transport  *libnet.NetworkTransport
	epoch      int
	epochCount int
}

func MakeConsensusModule(logger *log.Logger,
	transport *libnet.NetworkTransport) *ConsensusModule {
	cm := &ConsensusModule{}
	cm.logger = logger
	cm.transport = transport
	cm.epoch = 0
	cm.epochCount = 0
	go cm.readDataFromTransport()
	return nil
}

func (cm *ConsensusModule) readDataFromTransport() {
L:
	for {
		select {
		case <-cm.transport.Consume():
			////
		case <-cm.transport.Stopped():
			break L
		}
	}
	cm.logger.Println("Consensus module break")
}
