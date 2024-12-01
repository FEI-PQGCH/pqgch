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
	ClusterID  int    `json:"clusterId"`
	// Message content for user
	SenderName string `json:"sender"`
	Content    string `json:"content"`
}

var MessageTypeNames = map[int]string{
	LoginMsg:     "LoginMsg",
	BroadcastMsg: "BroadcastMsg",
	AkeAMsg:      "AkeAMsg",
	AkeBMsg:      "AkeBMsg",
	XiMsg:        "XiMsg",
}

func (m Message) MsgTypeName() string {
	return MessageTypeNames[m.MsgType]
}

const (
	LoginMsg = iota
	BroadcastMsg
	AkeAMsg
	AkeBMsg
	XiMsg
)

func SendMsg(conn net.Conn, msg Message) error {
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
