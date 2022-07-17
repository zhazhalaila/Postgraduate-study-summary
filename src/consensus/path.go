package consensus

import (
	"reflect"
	"sync"

	"github.com/zhazhalaila/PipelineBFT/src/message"
)

type Path struct {
	mu           sync.Mutex
	n            int
	f            int
	k            int
	batchSize    *int
	deliveredTxs map[int]map[int]*[][]byte
	qcs          [][]message.QuorumCert
}

// Construct new path
func NewPath(n, f, k int) *Path {
	p := &Path{}
	p.n = n
	p.f = f
	p.k = k
	p.batchSize = nil
	p.deliveredTxs = make(map[int]map[int]*[][]byte, p.k)
	p.qcs = make([][]message.QuorumCert, p.k)
	for i := 0; i < p.k; i++ {
		p.deliveredTxs[i] = make(map[int]*[][]byte)
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
func (p *Path) Add(qc message.QuorumCert, txs *[][]byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.qcs[qc.Round] = append(p.qcs[qc.Round], qc)

	if p.batchSize == nil {
		batchSize := len(*txs)
		p.batchSize = &batchSize
	}

	p.deliveredTxs[qc.Round][qc.Initiator] = txs
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
func (p *Path) GetTxs(round, initiator int) *[][]byte {
	return p.deliveredTxs[round][initiator]
}

// Get batch size
func (p *Path) GetBatchSize() int {
	return *p.batchSize
}
