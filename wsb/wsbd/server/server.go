package server

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"web-pubsub-example/wsb/wsbd/auth"
	"web-pubsub-example/wsb/wsbd/auth/jwt"
	"web-pubsub-example/wsb/wsbd/channel"
	"web-pubsub-example/wsb/wsbd/client"
	"web-pubsub-example/wsb/wsbd/message"
)

type Server struct {
	Clients client.Clients
	Channel *channel.Channel
	Broker  Broker
	Port    string
}

func NewServer(port string, b Broker) *Server {
	c := &channel.Channel{
		Chan: make(chan message.Message),
	}

	s := &Server{
		Clients: make(client.Clients),
		Channel: c,
		Broker:  b,
		Port:    port,
	}

	return s
}

func (s *Server) Start() error {
	go s.Broker.Handle(s.Channel)
	go s.handleMessages()

	http.Handle(
		"/websocket",
		jwt.Middleware(http.HandlerFunc(s.handleConnections)),
	)

	return http.ListenAndServe(s.Port, nil)
}

func (s *Server) handleMessages() {
	for {
		message := <-s.Channel.Chan
		if client, ok := s.Clients[message.To]; ok {
			ws := client.Conn
			err := ws.WriteMessage(1, []byte(message.Body))
			if err != nil {
				log.Printf("Error on WriteMessage %v!", err)
				ws.Close()
				delete(s.Clients, client.ID)
			}
		}
	}
}

func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	id := r.Context().Value(auth.SessionContextKey).(auth.Session).Identifier
	log.Printf("New client connected with ID %v!", id)

	upgrader := websocket.Upgrader{
		EnableCompression: true,
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error on Upgrade %v!", err)
		return
	}
	defer ws.Close()

	s.Clients[id] = &client.Client{
		ID:   id,
		Conn: ws,
	}

	for {
		messageType, message, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Error on ReadMessage %v!", err)
			break
		}

		log.Printf("Message %v received by server %v", messageType, string(message))
	}
}
