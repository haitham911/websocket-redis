package ws

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/go-redis/redis"
)

const (
	PlayerChannel = "Player-Channel"
	users         = "Player-users"
)

type Data struct {
	SessionId string      `json:"session_id,omitempty"`
	Action    string      `json:"action,omitempty"`
	Message   interface{} `json:"message,omitempty"`
	Sound     string      `json:"sound,omitempty"`
	Quantity  int         `json:"quantity,omitempty"`
	SpawnName string      `json:"spawn_name,omitempty"`
	Minutes   int         `json:"minutes,omitempty"`
	UserType  string      `json:"sender_user_type,omitempty"`
	Type      string      `json:"type,omitempty"`
	Name      string      `json:"name,omitempty"`
	Offer     *Offer      `json:"offer,omitempty"`
	Answer    *Answer     `json:"answer,omitempty"`
	Candidate *Candidate  `json:"candidate,omitempty"`
}
type Offer struct {
	Type string `json:"type"`
	Sdp  string `json:"sdp"`
}

// Answer struct
type Answer struct {
	Type string `json:"type"`
	Sdp  string `json:"sdp"`
}

// Candidate struct
type Candidate struct {
	Candidate     string `json:"candidate"`
	SdpMid        string `json:"sdpMid"`
	SdpMLineIndex int    `json:"sdpMLineIndex"`
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
	unregister            chan *Client
	RedisBroadcastChannel chan []byte

	// redis client
	redisClient *redis.Client
}

func NewHub(redisClient *redis.Client) *Hub {
	return &Hub{
		broadcast:             make(chan []byte),
		register:              make(chan *Client),
		unregister:            make(chan *Client),
		clients:               make(map[*Client]bool),
		RedisBroadcastChannel: make(chan []byte),

		redisClient: redisClient,
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
			go h.RemoveUser(client.SessionId)
		case message := <-h.RedisBroadcastChannel:

			h.PublishMessageToChannel(message)

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
				if client.UserType == data.UserType {
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
func (h *Hub) PublishMessageToChannel(message []byte) error {
	fmt.Println("PublishMessageToChannel", string(message))
	var data Data
	err := json.Unmarshal(message, &data)
	if err != nil {
		log.Println(err)
		return err
	}
	msg, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := h.redisClient.Publish(PlayerChannel, msg).Err(); err != nil {
		return err
	}
	return nil

}

func (h *Hub) ListenToPlayerChannelAndPublishToHub() {
	subscriber := h.redisClient.Subscribe(PlayerChannel)
	defer subscriber.Close()

	channel := subscriber.Channel()
	for msg := range channel {

		var data Data
		if err := json.Unmarshal([]byte(msg.Payload), &data); err != nil {
			log.Println("ListenToPlayerChannelAndPublishToHub", err)
			return
		}

		userMessage, _ := json.Marshal(data)
		h.broadcast <- []byte(userMessage)
	}
}

// CheckUserExists checks whether the user exists in the SET of active chat users
func (h *Hub) CheckUserExists(user string) (bool, error) {
	usernameTaken, err := h.redisClient.SIsMember(users, user).Result()
	if err != nil {
		return false, err
	}
	return usernameTaken, nil
}

// CreateUser creates a new user in the SET of active chat users
func (h *Hub) CreateUser(user string) error {
	err := h.redisClient.SAdd(users, user).Err()
	if err != nil {
		return err
	}
	return nil
}

// RemoveUser removes a user from the SET of active chat users
func (h *Hub) RemoveUser(user string) {
	err := h.redisClient.SRem(users, user).Err()
	if err != nil {
		log.Println("failed to remove user:", user)
		return
	}
	log.Println("removed user from redis:", user)
}

func AminSendMessage(hub *Hub, msg string) {
	var data Data
	// DropBall  PlaySound
	data.Action = "DropBall"
	data.Message = msg
	userMessage, _ := json.Marshal(data)
	hub.RedisBroadcastChannel <- []byte(userMessage)

}
func SendMessageToGame(hub *Hub, data Data) {
	userMessage, _ := json.Marshal(data)
	hub.broadcast <- []byte(userMessage)

}
