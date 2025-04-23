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
	queue shared.MessageQueue
}

// Cluster transport for communication between the leader and clients in its cluster.
type ClusterTransport struct {
	clients *Clients
	shared.BaseTransport
}

func NewClusterTransport(clients *Clients) *ClusterTransport {
	return &ClusterTransport{
		clients: clients,
	}
}

// 2-AKE messages are sent to specific clients, Xi and Key messages are broadcasted.
func (t *ClusterTransport) Send(msg shared.Message) {
	switch msg.Type {
	case shared.XiRiCommitmentMsg:
		broadcastToCluster(msg, t.clients)
	case shared.AkeAMsg:
		sendToClient(msg, t.clients)
	case shared.AkeBMsg:
		sendToClient(msg, t.clients)
	case shared.KeyMsg:
		broadcastToCluster(msg, t.clients)
	case shared.TextMsg:
		broadcastToCluster(msg, t.clients)
		broadcastToLeaders(msg)
	}
}

func (t *ClusterTransport) Receive(msg shared.Message) {
	t.MessageHandler(msg)
}

// Leader Transport for communication between leaders.
type LeaderTransport struct {
	shared.BaseTransport
}

func NewLeaderTransport() *LeaderTransport {
	return &LeaderTransport{}
}

// 2-AKE messages are sent only to specific leaders, the Xi message is broadcasted.
func (t *LeaderTransport) Send(msg shared.Message) {
	switch msg.Type {
	case shared.LeaderAkeAMsg:
		sendToLeader(config.GetNamesOrAddrs()[msg.ReceiverID], msg)
	case shared.LeaderAkeBMsg:
		sendToLeader(config.GetNamesOrAddrs()[msg.ReceiverID], msg)
	case shared.LeaderXiRiCommitmentMsg:
		broadcastToLeaders(msg)
	}
}

func (t *LeaderTransport) Receive(msg shared.Message) {
	t.MessageHandler(msg)
}

// Send msg to specific client in the cluster of this leader.
// If the client is not available, store the message in his queue.
func sendToClient(msg shared.Message, clients *Clients) {
	clients.mu.Lock()
	defer clients.mu.Unlock()

	client := &clients.cs[msg.ReceiverID]
	if client.conn == nil {
		client.queue.Add(msg)
		fmt.Printf("[DEBUG] Route: stored message\n")
		return
	}

	msg.Send(client.conn)
}

// Broadcast msg to every client in the cluster of this leader.
// If some of the clients are not available, store their messages in their queues.
func broadcastToCluster(msg shared.Message, clients *Clients) {
	clients.mu.Lock()
	defer clients.mu.Unlock()

	for i, c := range clients.cs {
		if c.index == msg.SenderID && msg.ClusterID == config.Index {
			continue
		}

		if c.conn == nil {
			clients.cs[i].queue.Add(msg)
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

// Broadcast msg to all the other leaders.
func broadcastToLeaders(msg shared.Message) {
	for i, addr := range config.GetNamesOrAddrs() {
		if i == config.Index {
			continue
		}
		sendToLeader(addr, msg)
	}
}

// Send msg to specific leader at address.
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
