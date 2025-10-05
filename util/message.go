package util

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
	LoginMsg:                "Login Message",
	TextMsg:                 "Text Message",
	AkeOneMsg:               "First AKE-2 Message",
	AkeTwoMsg:               "Second AKE-2 Message",
	XiRiCommitmentMsg:       "Xi, Randomness and Commitment Message",
	KeyMsg:                  "Encrypted Main Session Key Message",
	LeadAkeOneMsg:           "First Leader AKE-2 Message",
	LeadAkeTwoMsg:           "Second Leader AKE-2 Message",
	LeaderXiRiCommitmentMsg: "Leader Xi, Randomness and Commitment Message",
	QKDLeftKeyMsg:           "Left QKD Key Message",
	QKDRightKeyMsg:          "Right QKD Key Message",
	QKDClusterKeyMsg:        "Cluster QKD Key Message",
	QKDIDMsg:                "QKD ID Message",
}

func (m Message) TypeName() string {
	return MessageTypeNames[m.Type]
}

// All message types used in the application.
const (
	LoginMsg                = iota // Sent by cluster members to cluster leaders when logging on.
	TextMsg                        // Main Session Key encrypted text message.
	AkeOneMsg                      // First message of the 2-AKE protocol.
	AkeTwoMsg                      // Second message of the 2-AKE protocol.
	XiRiCommitmentMsg              // Message containing the Xi, Ri and Commitment values.
	KeyMsg                         // Message containg the encrypted Main Session Key. This key is encrypted using the Cluster Session Key.
	LeadAkeOneMsg                  // Same as AkeOneMsg, but for leaders.
	LeadAkeTwoMsg                  // Same as AkeTwoMsg, but for leaders.
	LeaderXiRiCommitmentMsg        // Same as XiRiCommitmentMsg, but for leaders.
	MainSessionKeyMsg              // Internal message used for transport from leader_protocol to cluster_protocol.
	QKDLeftKeyMsg                  // Response from the ETSI API server for Left Key.
	QKDRightKeyMsg                 // Response from the ETSI API server for Right Key.
	QKDClusterKeyMsg               // Response from the ETSI API server for Cluster Session Key.
	QKDIDMsg                       // Message containg the QKD ID for retrieving the second copy of the key.
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

func UniqueID() string {
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

func (reader *MessageReader) advance() {
	reader.nextMsg = nil
	reader.hasNext = false
	if reader.scanner.Scan() {
		var msg Message
		err := json.Unmarshal(reader.scanner.Bytes(), &msg)
		if err != nil {
			LogError(fmt.Sprint("Error unmarshaling message:", err))
		} else {
			reader.nextMsg = &msg
			reader.hasNext = true
		}
	} else {
		if err := reader.scanner.Err(); err != nil {
			LogError(fmt.Sprintf("Error reading from connection: %v", err))
		} else {
			LogInfo("Connection closed")
		}
	}
}

func (reader *MessageReader) HasMessage() bool {
	reader.advance()
	return reader.hasNext
}

func (reader *MessageReader) GetMessage() Message {
	if reader.nextMsg == nil {
		panic("getMsg called when no valid message is available")
	}
	msg := *reader.nextMsg
	return msg
}

func (m Message) IsEmpty() bool {
	return m == Message{}
}

// Message Tracker for tracking received messages.
// We track them so we do not process the same message twice.
// We distinguish messages according to their ID.
type MessageTracker struct {
	lock     sync.Mutex
	messages map[string]bool
}

func NewMessageTracker() *MessageTracker {
	return &MessageTracker{
		messages: make(map[string]bool),
	}
}

// AddMessage adds a message ID to the tracker and returns true if it was not already present.
func (tracker *MessageTracker) AddMessage(msgID string) bool {
	tracker.lock.Lock()
	defer tracker.lock.Unlock()
	if tracker.messages[msgID] {
		return false
	}
	tracker.messages[msgID] = true
	return true
}

// Message Queue for storing messages that we could not deliver.
type MessageQueue []Message

func (queue *MessageQueue) Add(msg Message) {
	*queue = append(*queue, msg)
}

func (queue *MessageQueue) Remove(msg Message) {
	tmp := MessageQueue{}
	for _, x := range *queue {
		if x.ID != msg.ID {
			tmp.Add(x)
		}
	}
	*queue = tmp
}
