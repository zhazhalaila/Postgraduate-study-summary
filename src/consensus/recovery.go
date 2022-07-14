package consensus

import (
	"encoding/json"
	"log"

	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	merkletree "github.com/zhazhalaila/PipelineBFT/src/merkleTree"
	"github.com/zhazhalaila/PipelineBFT/src/message"
)

type RECOVERY struct {
	// Global log
	logger    *log.Logger
	transport *libnet.NetworkTransport

	n         int
	f         int
	id        int
	epoch     int
	maxRound  int
	batchSize int

	// colleco all echos
	echos      map[int]map[int]map[int]map[int]*[]byte
	echosCount map[int]int
	// done
	done chan struct{}
	// stop decision instance
	stop chan bool
	// read data from channel
	inCh chan message.RECOVERY
	// return result to epoch module
	epochEvent chan Event
}

func MakeRecovery(
	logger *log.Logger,
	transport *libnet.NetworkTransport,
	n, f, id, epoch, maxRound int,
	epochEvent chan Event) *RECOVERY {
	r := &RECOVERY{}
	r.logger = logger
	r.transport = transport
	r.n = n
	r.f = f
	r.id = id
	r.epoch = epoch
	r.maxRound = maxRound
	r.echos = make(map[int]map[int]map[int]map[int]*[]byte)
	for i := 0; i < r.n; i++ {
		r.echos[i] = make(map[int]map[int]map[int]*[]byte)
		for j := 0; j < r.maxRound; j++ {
			r.echos[i][j] = make(map[int]map[int]*[]byte)
			for k := 0; k < r.n; k++ {
				r.echos[i][j][k] = make(map[int]*[]byte)
			}
		}
	}
	r.echosCount = make(map[int]int)

	r.done = make(chan struct{})
	r.stop = make(chan bool)
	r.inCh = make(chan message.RECOVERY, r.n)
	r.epochEvent = epochEvent
	go r.run()
	return r
}

func (r *RECOVERY) Input(msg message.RECOVERY) {
	r.inCh <- msg
}

func (r *RECOVERY) Stop() {
	close(r.stop)
}

func (r *RECOVERY) Done() <-chan struct{} {
	return r.done
}

func (r *RECOVERY) run() {
	defer func() {
		r.done <- struct{}{}
	}()

	for {
		select {
		case <-r.stop:
			return
		case msg := <-r.inCh:
			r.handleRecovery(msg)
		}
	}
}

func (r *RECOVERY) handleRecovery(recovery message.RECOVERY) {
	// Check all echo all valid
	for round, roundEchos := range recovery.Echos {
		for initiaitor, echo := range roundEchos {
			if !merkletree.MerkleTreeVerify(*echo.Shard, echo.RootHash, *echo.Branch, recovery.Sender) {
				r.logger.Printf("[Epoch:%d] [Peer:%d] receive invalid RECOVERY caused by [Round:%d] [Initiaitor:%d] [Sender:%d].\n",
					r.epoch, r.id, round, initiaitor, recovery.Sender)
				return
			}
		}
	}

	r.logger.Printf("[Epoch:%d] [Peer:%d] receiev valid RECOVERY msg from {Sender:%d, Producer:%d}.\n",
		r.epoch, r.id, recovery.Sender, recovery.Producer)

	r.echosCount[recovery.Producer]++

	for round, roundEchos := range recovery.Echos {
		for initiator, echo := range roundEchos {
			r.echos[recovery.Producer][round][initiator][recovery.Sender] = echo.Shard
		}
	}

	for producer := range r.echosCount {
		if r.echosCount[producer] == 2*r.f+1 {
			r.decode(producer)
		}
	}
}

func (r *RECOVERY) decode(producer int) {
	producerEchos := r.echos[producer]
	for round, roundEchos := range producerEchos {
		for initiaitor, innerEcho := range roundEchos {
			shards := make([][]byte, r.n)
			for index, shard := range innerEcho {
				shards[index] = *shard
			}
			if !r.validArray(shards) {
				continue
			}
			r.logger.Printf("[Epoch:%d] [Round:%d] [Initiator:%d] decode array.\n",
				r.epoch, round, initiaitor)
			decode, err := ECDecode(r.f+1, r.n-(r.f+1), shards)
			if err != nil {
				r.logger.Printf("[Epoch:%d] [Round:%d] decode for [Initiator:%d] error.\n",
					r.epoch, round, initiaitor)
			}
			var transaction [][]byte
			err = json.Unmarshal(decode, &transaction)
			if err != nil {
				r.logger.Printf("[Epoch:%d] [Round:%d] unmarshal for  [Initiator:%d] error.\n",
					r.epoch, round, initiaitor)
			}
			r.logger.Printf("[Epoch:%d] [Round:%d] [Initiator:%d] decode transaction success.\n",
				r.epoch, round, initiaitor)
			// r.logger.Println(transaction)
			r.batchSize = len(transaction)
		}
	}

	select {
	case <-r.stop:
		return
	default:
		recoveryOut := RecoveryOutput{batchSize: r.batchSize}
		r.epochEvent <- Event{eventType: DeliverRecovery, payload: recoveryOut}
	}
}

func (r *RECOVERY) validArray(decodeArray [][]byte) bool {
	count := 0
	for _, a := range decodeArray {
		if len(a) > 0 {
			count++
		}
	}
	return count > r.f+1
}
