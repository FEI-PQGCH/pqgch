package shared

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
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
	LoginMsg:                "LoginMsg",
	TextMsg:                 "TextMsg",
	AkeAMsg:                 "AkeAMsg",
	AkeBMsg:                 "AkeBMsg",
	XiRiCommitmentMsg:       "XiRiCommitmentMsg",
	KeyMsg:                  "KeyMsg",
	LeaderAkeAMsg:           "LeaderAkeAMsg",
	LeaderAkeBMsg:           "LeaderAkeBMsg",
	LeaderXiRiCommitmentMsg: "LeaderXiRiCommitmentMsg",
}

func (m Message) TypeName() string {
	return MessageTypeNames[m.Type]
}

const (
	LoginMsg = iota
	TextMsg
	AkeAMsg
	AkeBMsg
	XiRiCommitmentMsg
	KeyMsg
	LeaderAkeAMsg
	LeaderAkeBMsg
	LeaderXiRiCommitmentMsg
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

func GenerateUniqueID() string {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
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
			fmt.Printf("[INFO] Connection closed\n")
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

// Message Tracker for tracking received messages.
// We track them so we do not process the same message twice.
// We distinguish messages according to their ID.
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

// Message Queue for storing messages that we could not deliver.
type MessageQueue []Message

func (mq *MessageQueue) Add(msg Message) {
	*mq = append(*mq, msg)
}

func (mq *MessageQueue) Remove(msg Message) {
	tmp := MessageQueue{}
	for _, x := range *mq {
		if x.ID != msg.ID {
			tmp.Add(x)
		}
	}
	*mq = tmp
}
