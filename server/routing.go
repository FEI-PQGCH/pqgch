package main

import (
	"fmt"
	"net"
	"pqgch/shared"
	"sync"
	"time"
)

type Clients struct {
	cs []Client
	mu sync.Mutex
}

type Client struct {
	name  string
	conn  net.Conn
	index int
	queue MessageQueue
}

type MessageQueue []shared.Message

func (mq *MessageQueue) add(msg shared.Message) {
	*mq = append(*mq, msg)
}

func (mq *MessageQueue) remove(msg shared.Message) {
	tmp := MessageQueue{}
	for _, x := range *mq {
		if x.ID != msg.ID {
			tmp.add(x)
		}
	}
	*mq = tmp
}

type ClusterTransport struct {
	clients *Clients
	shared.BaseTransport
}

func NewClusterTransport(clients *Clients) *ClusterTransport {
	return &ClusterTransport{
		clients: clients,
	}
}

func (t *ClusterTransport) Send(msg shared.Message) {
	switch msg.Type {
	case shared.XiMsg:
		broadcastToCluster(msg, t.clients)
	case shared.AkeAMsg:
		sendToClient(msg, t.clients)
	case shared.AkeBMsg:
		sendToClient(msg, t.clients)
	case shared.KeyMsg:
		broadcastToCluster(msg, t.clients)
	}
}

func (t *ClusterTransport) Receive(msg shared.Message) {
	t.MessageHandler(msg)
}

type LeaderTransport struct {
	shared.BaseTransport
}

func NewLeaderTransport() *LeaderTransport {
	return &LeaderTransport{}
}

func (t *LeaderTransport) Send(msg shared.Message) {
	switch msg.Type {
	case shared.LeaderAkeAMsg:
		sendToLeader(config.GetNamesOrAddrs()[msg.ReceiverID], msg)
	case shared.LeaderAkeBMsg:
		sendToLeader(config.GetNamesOrAddrs()[msg.ReceiverID], msg)
	case shared.LeaderXiMsg:
		broadcastToLeaders(msg)
	}
}

func (t *LeaderTransport) Receive(msg shared.Message) {
	t.MessageHandler(msg)
}

func sendToClient(msg shared.Message, clients *Clients) {
	clients.mu.Lock()
	defer clients.mu.Unlock()

	client := &clients.cs[msg.ReceiverID]
	if client.conn == nil {
		client.queue.add(msg)
		fmt.Printf("[DEBUG] Route: stored message\n")
		return
	}

	msg.Send(client.conn)
}

func broadcastToCluster(msg shared.Message, clients *Clients) {
	clients.mu.Lock()
	defer clients.mu.Unlock()

	for i, c := range clients.cs {
		if c.index == msg.SenderID && msg.ClusterID == config.Index {
			continue
		}

		if c.conn == nil {
			clients.cs[i].queue.add(msg)
			continue
		}

		err := msg.Send(c.conn)
		if err != nil {
			fmt.Println("error sending message to client:", err)
			c.conn.Close()
			return
		}
	}
	fmt.Printf("[ROUTE] Broadcasted message %s from %s\n", msg.TypeName(), msg.SenderName)
}

func broadcastToLeaders(msg shared.Message) {
	for i, addr := range config.GetNamesOrAddrs() {
		if i == config.Index {
			continue
		}
		sendToLeader(addr, msg)
	}
}

func sendToLeader(address string, msg shared.Message) {
	var conn net.Conn
	for {
		var err error
		conn, err = net.Dial("tcp", address)
		if err != nil {
			fmt.Printf("[ERROR] Leader (%s) connection error: %v. Retrying...\n", address, err)
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	fmt.Printf("[INFO] Sending message %s to Leader %s\n", msg.TypeName(), address)

	msg.Send(conn)
}
