package shared

type Message struct {
	// Routing metadata
	MsgID      string `json:"msg_id"`
	SenderID   int    `json:"send_id"`
	ReceiverID int    `json:"recv_id"`
	MsgType    int    `json:"msg_type"`
	// Message content for user
	SenderName string `json:"sender"`
	Content    string `json:"content"`
}

const (
	MsgLogin = iota
	MsgIntra
	MsgBroadcast
)
