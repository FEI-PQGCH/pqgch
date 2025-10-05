package util

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

type MessageSender interface {
	Send(msg Message)
}

type TCPTransport struct {
	conn        net.Conn
	mu          sync.Mutex
	receiveChan chan Message
}

func NewTCPTransport(address string, receiveChan chan Message) (*TCPTransport, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	t := &TCPTransport{
		conn:        conn,
		receiveChan: receiveChan,
	}

	go t.listen()

	return t, nil
}

func (t *TCPTransport) listen() {
	reader := NewMessageReader(t.conn)

	for reader.HasMessage() {
		msg := reader.GetMessage()
		if msg.Type != TextMsg {
			LogRouteWithNames("RECEIVED", msg.TypeName(), "from", msg.SenderName)
		}
		t.receiveChan <- msg
	}
}

func (t *TCPTransport) Send(msg Message) {
	t.mu.Lock()
	defer t.mu.Unlock()

	msgData, err := json.Marshal(msg)
	if err != nil {
		LogError(fmt.Sprintf("Error marshaling message: %v", err))
		return
	}

	msgData = append(msgData, '\n')

	_, err = t.conn.Write(msgData)
	if err != nil {
		LogError(fmt.Sprintf("Error sending message: %v\n", err))
	}
}
