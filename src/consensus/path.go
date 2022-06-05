package consensus

type Path struct {
	values []interface{}
	size   int
}

// Construct new path
func (p *Path) NewPath() *Path {
	path := &Path{}
	path.size = 0
	path.values = make([]interface{}, path.size)
	return path
}

// Add path
func (p *Path) Add(value interface{}) {
	p.values = append(p.values, value)
	p.size++
}

// Get path size
func (p *Path) Size() int {
	return p.size
}
