package util

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

type MessageSender interface {
	Send(msg Message)
}

type TCPTransport struct {
	conn        net.Conn
	mu          sync.Mutex
	receiveChan chan Message
}

func NewTCPTransport(address string, receiveChan chan Message, clientID, clusterID int) (*TCPTransport, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	t := &TCPTransport{
		conn:        conn,
		receiveChan: receiveChan,
	}

	go t.listen()
	go t.pingPong(clientID, clusterID)

	return t, nil
}

func (t *TCPTransport) pingPong(clientID, clusterID int) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ping := Message{
			SenderID:  clientID,
			ClusterID: clusterID,
			Type:      Ping,
		}
		t.Send(ping)
	}
}

func (t *TCPTransport) listen() {
	reader := NewMessageReader(t.conn)

	for reader.HasMessage() {
		msg := reader.GetMessage()
		if msg.Type == Pong {
			continue
		}
		if msg.Type == Error {
			ExitWithMsg("From routing server: " + msg.Content)
		}
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
		ExitWithMsg(fmt.Sprintf("failed to send message: %v", err))
	}
}
