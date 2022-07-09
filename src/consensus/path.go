package consensus

import (
	"reflect"
	"sync"

	"github.com/zhazhalaila/PipelineBFT/src/message"
)

type Path struct {
	mu  sync.Mutex
	n   int
	f   int
	k   int
	qcs [][]message.QuorumCert
}

// Construct new path
func NewPath(n, f, k int) *Path {
	p := &Path{}
	p.n = n
	p.f = f
	p.k = k
	p.qcs = make([][]message.QuorumCert, p.k)
	for i := 0; i < p.k; i++ {
		p.qcs[i] = make([]message.QuorumCert, 0)
	}
	return p
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
func (p *Path) Add(qc message.QuorumCert) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.qcs[qc.Round] = append(p.qcs[qc.Round], qc)
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
