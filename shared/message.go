package shared

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

type Message struct {
	// Routing metadata
	ID         string `json:"id"`
	SenderID   int    `json:"sendId"`
	ReceiverID int    `json:"recvId"`
	Type       int    `json:"type"`
	ClusterID  int    `json:"clusterId"`
	// Message content for user
	SenderName string `json:"sender"`
	Content    string `json:"content"`
}

var MessageTypeNames = map[int]string{
	LoginMsg:      "LoginMsg",
	BroadcastMsg:  "BroadcastMsg",
	AkeAMsg:       "AkeAMsg",
	AkeBMsg:       "AkeBMsg",
	XiMsg:         "XiMsg",
	LeaderAkeAMsg: "LeaderAkeAMsg",
	LeaderAkeBMsg: "LeaderAkeBMsg",
	LeaderXiMsg:   "LeaderXiMsg",
}

func (m Message) TypeName() string {
	return MessageTypeNames[m.Type]
}

const (
	LoginMsg = iota
	BroadcastMsg
	AkeAMsg
	AkeBMsg
	XiMsg
	LeaderAkeAMsg
	LeaderAkeBMsg
	LeaderXiMsg
)

func (m Message) Send(conn net.Conn) error {
	msgData, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("error marshaling message: %w", err)
	}

	msgData = append(msgData, '\n')

	_, err = conn.Write(msgData)
	if err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	return nil
}

type MessageReader struct {
	scanner *bufio.Scanner
	nextMsg *Message
	hasNext bool
}

func NewMessageReader(conn net.Conn) *MessageReader {
	reader := &MessageReader{
		scanner: bufio.NewScanner(conn),
	}
	return reader
}

func (mr *MessageReader) advance() {
	mr.nextMsg = nil
	mr.hasNext = false
	if mr.scanner.Scan() {
		var msg Message
		err := json.Unmarshal(mr.scanner.Bytes(), &msg)
		if err != nil {
			fmt.Println("error unmarshaling message:", err)
		} else {
			mr.nextMsg = &msg
			mr.hasNext = true
		}
	} else {
		if err := mr.scanner.Err(); err != nil {
			fmt.Println("error reading from connection:", err)
		} else {
			fmt.Println("connection closed")
		}
	}
}

func (mr *MessageReader) HasMessage() bool {
	mr.advance()
	return mr.hasNext
}

func (mr *MessageReader) GetMessage() Message {
	if mr.nextMsg == nil {
		panic("getMsg called when no valid message is available")
	}
	msg := *mr.nextMsg
	return msg
}

func (m Message) IsEmpty() bool {
	return m == Message{}
}

type MessageTracker struct {
	mu       sync.Mutex
	messages map[string]bool
}

func NewMessageTracker() *MessageTracker {
	return &MessageTracker{
		messages: make(map[string]bool),
	}
}

// AddMessage adds a message ID to the tracker and returns true if it was not already present.
func (mt *MessageTracker) AddMessage(msgID string) bool {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if mt.messages[msgID] {
		return false
	}
	mt.messages[msgID] = true
	return true
}
