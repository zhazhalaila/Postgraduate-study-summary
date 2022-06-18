package consensus

import (
	"log"

	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	"github.com/zhazhalaila/PipelineBFT/src/message"
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
	return cm
}

func (cm *ConsensusModule) Run() {
	go cm.readDataFromTransport()
}

func (cm *ConsensusModule) readDataFromTransport() {
L:
	for {
		select {
		case cmd := <-cm.transport.Consume():
			log.Println("Consensus: ", cmd)
			cm.handleCommand(cmd)
		case <-cm.transport.Stopped():
			break L
		}
	}
	cm.logger.Println("Consensus module break")
}

func (cm *ConsensusModule) handleCommand(cmd interface{}) {
	switch cmd.(type) {
	case message.PrePrepare:
		log.Println(cmd.(message.PrePrepare).Initiator)
	}
}
