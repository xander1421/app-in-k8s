package webtransport

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/quic-go/webtransport-go"
)

// Handler handles WebTransport connections
type Handler struct {
	upgrader *webtransport.Server
}

// NewHandler creates a new WebTransport handler
func NewHandler() *Handler {
	return &Handler{
		upgrader: &webtransport.Server{},
	}
}

// ServeHTTP handles WebTransport upgrade requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, err := h.upgrader.Upgrade(w, r)
	if err != nil {
		log.Printf("WebTransport upgrade failed: %v", err)
		http.Error(w, "WebTransport upgrade failed", http.StatusBadRequest)
		return
	}

	go h.handleSession(session)
}

// handleSession manages a WebTransport session
func (h *Handler) handleSession(session *webtransport.Session) {
	ctx := session.Context()
	defer session.CloseWithError(0, "session closed")

	// Handle incoming streams
	go func() {
		for {
			stream, err := session.AcceptStream(ctx)
			if err != nil {
				log.Printf("Failed to accept stream: %v", err)
				return
			}
			go h.handleStream(stream)
		}
	}()

	// Handle incoming datagrams
	go func() {
		for {
			data, err := session.ReceiveDatagram(ctx)
			if err != nil {
				log.Printf("Failed to receive datagram: %v", err)
				return
			}
			h.handleDatagram(data)
		}
	}()

	<-ctx.Done()
}

// handleStream processes a WebTransport stream
func (h *Handler) handleStream(stream *webtransport.Stream) {
	defer stream.Close()

	// Read message from stream
	var message map[string]interface{}
	decoder := json.NewDecoder(stream)
	if err := decoder.Decode(&message); err != nil {
		log.Printf("Failed to decode stream message: %v", err)
		return
	}

	log.Printf("Received stream message: %+v", message)

	// Echo response
	response := map[string]interface{}{
		"type": "response",
		"data": message,
	}

	encoder := json.NewEncoder(stream)
	if err := encoder.Encode(response); err != nil {
		log.Printf("Failed to encode stream response: %v", err)
	}
}

// handleDatagram processes a WebTransport datagram
func (h *Handler) handleDatagram(data []byte) {
	var message map[string]interface{}
	if err := json.Unmarshal(data, &message); err != nil {
		log.Printf("Failed to decode datagram: %v", err)
		return
	}

	log.Printf("Received datagram: %+v", message)
}

// SendMessage sends a message to the WebTransport session
func (h *Handler) SendMessage(session *webtransport.Session, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	return session.SendDatagram(data)
}

// BroadcastMessage broadcasts a message to all connected sessions
func (h *Handler) BroadcastMessage(message interface{}) {
	// This would require maintaining a list of active sessions
	// For now, this is a placeholder
	log.Printf("Broadcasting message: %+v", message)
}