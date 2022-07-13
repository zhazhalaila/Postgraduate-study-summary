package consensus

import (
	"reflect"
	"sync"

	"github.com/zhazhalaila/PipelineBFT/src/message"
)

type Path struct {
	mu    sync.Mutex
	n     int
	f     int
	k     int
	echos map[int]map[int]message.ECHO
	qcs   [][]message.QuorumCert
}

// Construct new path
func NewPath(n, f, k int) *Path {
	p := &Path{}
	p.n = n
	p.f = f
	p.k = k
	p.echos = make(map[int]map[int]message.ECHO, p.k)
	p.qcs = make([][]message.QuorumCert, p.k)
	for i := 0; i < p.k; i++ {
		p.echos[i] = make(map[int]message.ECHO)
		p.qcs[i] = make([]message.QuorumCert, 0)
	}
	return p
}

// Get Path len
func (p *Path) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	l := 0
	for i := 0; i < p.k; i++ {
		l += len(p.qcs[i])
	}
	return l
}

// Check qc has been cache
func (p *Path) Exist(qc message.QuorumCert) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, b := range p.qcs[qc.Round] {
		if reflect.DeepEqual(qc, b) {
			return true
		}
	}
	return false
}

// Add path
func (p *Path) Add(qc message.QuorumCert, rootHash [32]byte, branch *[][32]byte, shard *[]byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.qcs[qc.Round] = append(p.qcs[qc.Round], qc)
	e := message.ECHO{
		RootHash: rootHash,
		Branch:   branch,
		Shard:    shard,
	}
	p.echos[qc.Round][qc.Initiator] = e
}

// Check there are at least n-f qc for each r
func (p *Path) RecvThreshold() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := true
	for i := 0; i < p.k; i++ {
		if len(p.qcs[i]) < p.n-p.f {
			result = false
		}
	}
	return result
}

// Get qcs
func (p *Path) GetQC() [][]message.QuorumCert {
	p.mu.Lock()
	defer p.mu.Unlock()

	qcCopy := p.qcs
	return qcCopy
}

// Get echo
func (p *Path) GetEcho(round, initiator int) message.ECHO {
	return p.echos[round][initiator]
}
