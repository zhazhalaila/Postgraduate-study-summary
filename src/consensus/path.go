package consensus

import (
	"encoding/json"
	"reflect"

	"github.com/zhazhalaila/PipelineBFT/src/message"
)

type Path struct {
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

// Serialize path
func SerializePath(p *Path) ([]byte, error) {
	qcs := p.qcs
	qcsBytes, err := json.Marshal(qcs)
	if err != nil {
		return nil, err
	}
	return qcsBytes, nil
}

// Deserialize path
func DeserializePath(qcsByte []byte) (*Path, error) {
	var qcs [][]message.QuorumCert
	err := json.Unmarshal(qcsByte, &qcs)
	if err != nil {
		return nil, err
	}
	p := &Path{}
	p.qcs = qcs
	return p, nil
}

// Check qc has been cache
func (p *Path) Exist(r int, qc message.QuorumCert) bool {
	for _, b := range p.qcs[r] {
		if reflect.DeepEqual(qc, b) {
			return true
		}
	}
	return false
}

// Add path
func (p *Path) Add(r int, qc message.QuorumCert) {
	p.qcs[r] = append(p.qcs[r], qc)
}

// Check there are at least n-f qc for each r
func (p *Path) RecvThreshold() bool {
	result := true
	for i := 0; i < p.k; i++ {
		if len(p.qcs[i]) < p.n-p.f {
			result = false
		}
	}
	return result
}
