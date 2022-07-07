package message

// Network message entrance
type Entrance struct {
	ModuleType uint8
	Epoch      int
	Payload    interface{}
}
