package controller

import (
	"net/http"

	"hpe-recipe/internal/integration/websocket"
)

type WSController struct {
	hub *websocket.Hub
}

func NewWSController(hub *websocket.Hub) *WSController {
	return &WSController{
		hub: hub,
	}
}

func (c *WSController) ServeWS(w http.ResponseWriter, r *http.Request) {
	c.hub.ServeWS(w, r)
}
