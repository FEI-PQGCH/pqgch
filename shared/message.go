package shared

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
