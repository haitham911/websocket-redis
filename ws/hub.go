// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ws

import (
	"encoding/json"
	"log"
)

type Data struct {
	SessionId string      `json:"session_id"`
	Action    string      `json:"action"`
	Message   interface{} `json:"message"`
	Sound     string      `json:"sound"`
	Quantity  int         `json:"quantity"`
	SpawnName string      `json:"spawn_name"`
	Minutes   int         `json:"minutes"`
}

// Hub maintains the set of active clients and broadcasts messages to the
// clients.

type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			var data Data
			err := json.Unmarshal(message, &data)
			if err != nil {
				log.Println(err)
				return
			}
			for client := range h.clients {
				if client.SessionId != data.SessionId {
					continue
				}

				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
func AminSendMessage(hub *Hub, msg string) {
	var data Data
	// DropBall  PlaySound
	data.Action = "DropBall"
	data.Message = msg
	userMessage, _ := json.Marshal(data)
	hub.broadcast <- []byte(userMessage)

}
func SendMessageToGame(hub *Hub, data Data) {
	userMessage, _ := json.Marshal(data)
	hub.broadcast <- []byte(userMessage)

}
