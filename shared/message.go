package shared

import (
	"encoding/json"
	"fmt"
	"net"
)

type Message struct {
	// Routing metadata
	MsgID      string `json:"msgId"`
	SenderID   int    `json:"sendId"`
	ReceiverID int    `json:"recvId"`
	MsgType    int    `json:"msgType"`
	// Message content for user
	SenderName string `json:"sender"`
	Content    string `json:"content"`
}

const (
	MsgLogin = iota
	MsgIntra
	MsgBroadcast
)

func SendMessage(conn net.Conn, msg Message) error {
	msgData, err := json.Marshal(msg)
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
