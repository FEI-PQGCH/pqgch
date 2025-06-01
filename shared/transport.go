package shared

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

type Transport interface {
	Send(msg Message)
	SetMessageHandler(handler func(Message))
}

type BaseTransport struct {
	MessageHandler func(Message)
}

func (t *BaseTransport) SetMessageHandler(handler func(Message)) {
	t.MessageHandler = handler
}

type TCPTransport struct {
	conn net.Conn
	BaseTransport
	cond *sync.Cond
	mu   sync.Mutex
}

func NewTCPTransport(address string) (*TCPTransport, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("[ERROR] Failed to connect to server: %w", err)
	}

	t := &TCPTransport{
		conn: conn,
	}
	t.cond = sync.NewCond(&t.mu)

	go t.listen()

	return t, nil
}

func (t *TCPTransport) SetMessageHandler(handler func(Message)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.MessageHandler = handler
	t.cond.Broadcast()
}

func (t *TCPTransport) Send(msg Message) {
	t.mu.Lock()
	defer t.mu.Unlock()

	msgData, err := json.Marshal(msg)
	if err != nil {
		fmt.Println("[ERROR] Error marshaling message:", err)
		return
	}

	msgData = append(msgData, '\n')

	_, err = t.conn.Write(msgData)
	if err != nil {
		fmt.Println("[ERROR] Error sending message:", err)
	}
}

func (t *TCPTransport) listen() {
	reader := NewMessageReader(t.conn)

	t.mu.Lock()
	for t.MessageHandler == nil {
		t.cond.Wait()
	}
	handler := t.MessageHandler
	t.mu.Unlock()

	for reader.HasMessage() {
		msg := reader.GetMessage()
		if msg.Type != TextMsg {
			fmt.Printf("[ROUTE] Received %s from %s \n", msg.TypeName(), msg.SenderName)
		}
		handler(msg)
	}
}

func (t *TCPTransport) Close() {
	t.conn.Close()
}
