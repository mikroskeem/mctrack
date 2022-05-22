package mcping

import "encoding/json"

// PingResponse contains all known fields of the ping response packet
type PingResponse struct {
	Version struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	} `json:"version"`
	Players struct {
		Max    int            `json:"max"`
		Online int            `json:"online"`
		Sample []PlayerSample `json:"sample"`
	} `json:"players"`
	Description json.RawMessage `json:"description"`
	Favicon     string          `json:"favicon"`

	Latency uint `json:"latency"`
}
