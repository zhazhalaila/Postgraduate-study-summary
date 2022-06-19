package consensus

import (
	"log"

	"github.com/zhazhalaila/PipelineBFT/src/message"
)

const (
	PBCOUTPUT = iota
	PRODUCER  = iota
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
	// k reference how many rounds concurrently process in each epoch
	maxRound int

	path *Path
	pbcs map[int]map[int]*PBC

	event chan Event
	stop  chan bool
	inCh  chan interface{}
}

func NewEpoch(
	logger *log.Logger,
	n, f, id, maxRound int,
) *Epoch {
	e := &Epoch{}
	e.logger = logger
	e.n = n
	e.f = f
	e.id = id
	e.maxRound = maxRound
	e.path = NewPath(e.n, e.f, e.maxRound)
	e.pbcs = make(map[int]map[int]*PBC, e.n)
	for i := 0; i < maxRound; i++ {
		e.pbcs[i] = make(map[int]*PBC, e.n)
	}
	e.event = make(chan Event, e.n*e.maxRound)
	e.inCh = make(chan interface{}, e.n*e.n*e.maxRound)
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
			e.handleMsg(msg)
		case event := <-e.event:
			e.handleEvent(event)
		}
	}
}

func (e *Epoch) handleMsg(msg interface{}) {
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
