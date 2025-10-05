package util

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
)

type Message struct {
	// Routing metadata
	SenderID   int `json:"sendId"`
	ReceiverID int `json:"recvId"`
	Type       int `json:"type"`
	ClusterID  int `json:"clusterId"`
	// Message content for user
	SenderName string `json:"sender"`
	Content    string `json:"content"`
}

var MessageTypeNames = map[int]string{
	MemberAuthMsg:           "Member Authentication Message",
	LeaderAuthMsg:           "Leader Authentication Message",
	TextMsg:                 "Text Message",
	AkeOneMsg:               "First AKE-2 Message",
	AkeTwoMsg:               "Second AKE-2 Message",
	XiRiCommitmentMsg:       "Xi, Ri and Commitment Message",
	KeyMsg:                  "Encrypted Main Session Key Message",
	LeadAkeOneMsg:           "First Leader AKE-2 Message",
	LeadAkeTwoMsg:           "Second Leader AKE-2 Message",
	LeaderXiRiCommitmentMsg: "Leader Xi, Ri and Commitment Message",
	QKDLeftKeyMsg:           "Left QKD Key Message",
	QKDRightKeyMsg:          "Right QKD Key Message",
	QKDClusterKeyMsg:        "Cluster QKD Key Message",
	QKDIDLeaderMsg:          "QKD ID Message",
	QKDIDMemberMsg:          "QKD ID Message",
}

func (m Message) TypeName() string {
	return MessageTypeNames[m.Type]
}

// All message types used in the application.
const (
	MemberAuthMsg = iota
	LeaderAuthMsg
	TextMsg                 // Main Session Key encrypted text message.
	AkeOneMsg               // First message of the 2-AKE protocol.
	AkeTwoMsg               // Second message of the 2-AKE protocol.
	XiRiCommitmentMsg       // Message containing the Xi, Ri and Commitment values.
	KeyMsg                  // Message containg the encrypted Main Session Key. This key is encrypted using the Cluster Session Key.
	LeadAkeOneMsg           // Same as AkeOneMsg, but for leaders.
	LeadAkeTwoMsg           // Same as AkeTwoMsg, but for leaders.
	LeaderXiRiCommitmentMsg // Same as XiRiCommitmentMsg, but for leaders.
	QKDIDLeaderMsg          // Message containg the QKD ID for retrieving the second copy of the key.
	QKDIDMemberMsg
	Ping
	Pong
	Error

	QKDLeftKeyMsg     // Response from the ETSI API server for Left Key.
	QKDRightKeyMsg    // Response from the ETSI API server for Right Key.
	QKDClusterKeyMsg  // Response from the ETSI API server for Cluster Session Key.
	MainSessionKeyMsg // Internal message used for transport from leader_protocol to cluster_protocol.
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
