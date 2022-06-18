package consensus

import (
	"log"

	"github.com/zhazhalaila/PipelineBFT/src/message"
)

const (
	PBCOUTPUT = iota
	PRODUCER  = iota
	ABAOUTPUT = iota
	ABASTOP   = iota
)

type Event struct {
	eventType  int
	r          int
	instanceId int
	qc         message.QuorumCert
	producer   int
	decision   int
}

type Epoch struct {
	// Global log
	logger *log.Logger

	n  int
	f  int
	id int
	// k reference how many transactions a node can broadcast concurrently in each epoch
	k int

	path *Path
	pbcs map[int]map[int]*PBC

	event chan Event
	stop  chan bool
	inCh  chan interface{}
}

func NewEpoch(
	logger *log.Logger,
	n, f, id, k int,
) *Epoch {
	e := &Epoch{}
	e.logger = logger
	e.n = n
	e.f = f
	e.id = id
	e.k = k
	e.path = NewPath(e.n, e.f, e.k)
	e.pbcs = make(map[int]map[int]*PBC, e.n)
	for i := 0; i < k; i++ {
		e.pbcs[i] = make(map[int]*PBC, e.k)
	}
	e.event = make(chan Event, e.n*e.k)
	e.inCh = make(chan interface{}, e.n*e.n*e.k)
	go e.run()
	return e
}

func (e *Epoch) run() {
L:
	for {
		select {
		case <-e.stop:
			break L
		case msg := <-e.inCh:
			e.handleCommand(msg)
		case event := <-e.event:
			e.handleEvent(event)
		}
	}
}

func (e *Epoch) handleCommand(msg interface{}) {
}

func (e *Epoch) handleEvent(event Event) {
	switch event.eventType {
	case PBCOUTPUT:
		e.handlePBCOut(event.r, event.qc)
	}
}

func (e *Epoch) handlePBCOut(r int, qc message.QuorumCert) {
	if e.path.Exist(r, qc) {
		return
	}

	e.path.Add(r, qc)
	if e.path.RecvThreshold() {
		// broadcast to all
	}
}
