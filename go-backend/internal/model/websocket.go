package model

// WSEvent defines the real-time event envelope broadcast across WebSocket sessions.
type WSEvent struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}
