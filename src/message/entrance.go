package message

import "encoding/json"

// Network message entrance
type Entrance struct {
	ModuleType uint8
	Epoch      int
	Payload    json.RawMessage
}

func GenEntrance(moduleType uint8, epoch int, payload []byte) Entrance {
	entrance := Entrance{
		ModuleType: moduleType,
		Epoch:      epoch,
		Payload:    payload,
	}
	return entrance
}
