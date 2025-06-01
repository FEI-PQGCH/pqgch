package main

import (
	"fmt"
	"net"
	"pqgch/shared"
	"sync"
	"time"
)

type Clients struct {
	data []Client
	lock sync.Mutex
}

type Client struct {
	name  string
	conn  net.Conn
	index int
	queue shared.MessageQueue
}

func newClients(config shared.ConfigAccessor) *Clients {
	var clients Clients

	for i, addr := range config.GetNamesOrAddrs() {
		if i == config.GetIndex() {
			continue
		}
		client := Client{
			name:  addr,
			conn:  nil,
			index: i,
		}
		clients.data = append(clients.data, client)
	}

	return &clients
}

// Save the connection of this client, making him online.
func (clients *Clients) makeOnline(id int, conn net.Conn) {
	clients.lock.Lock()
	defer clients.lock.Unlock()
	clients.data[id].conn = conn
}

// Clean up the connection after a disconnected client.
func (clients *Clients) makeOffline(id int) {
	clients.lock.Lock()
	defer clients.lock.Unlock()
	fmt.Println("[INFO] Client disconnected:", clients.data[id].conn.RemoteAddr())
	clients.data[id].conn.Close()
}

// Send queued messages to the client with id.
func (clients *Clients) sendQueued(id int) {
	clients.lock.Lock()
	defer clients.lock.Unlock()

	for _, msg := range clients.data[id].queue {
		msg.Send(clients.data[id].conn)
		clients.data[id].queue.Remove(msg)
	}
}

// Send a message to a specific client.
// If the client is not available, store the message in his queue.
func (clients *Clients) send(msg shared.Message) {
	clients.lock.Lock()
	defer clients.lock.Unlock()

	client := &clients.data[msg.ReceiverID]
	if client.conn == nil {
		client.queue.Add(msg)
		fmt.Printf("[ROUTE] Stored message\n")
		return
	}
	msg.Send(client.conn)
}

// Broadcast message to all clients
// If some of the clients are not available, store their messages in their queues.
func (clients *Clients) broadcast(msg shared.Message) {
	clients.lock.Lock()
	defer clients.lock.Unlock()

	for i, c := range clients.data {
		if c.index == msg.SenderID && msg.ClusterID == config.Index {
			continue
		}

		if c.conn == nil {
			clients.data[i].queue.Add(msg)
			continue
		}

		err := msg.Send(c.conn)
		if err != nil {
			fmt.Println("[ERROR] sending message to client:", err)
			c.conn.Close()
			return
		}
	}
	fmt.Printf("[ROUTE] Broadcasted message %s from %s\n", msg.TypeName(), msg.SenderName)
}

type ServerTransport interface {
	receive(shared.Message)
}

func transportManager(t ServerTransport, msgs <-chan shared.Message) {
	for msg := range msgs {
		t.receive(msg)
	}
}

// Cluster transport for communication between the leader and clients in its cluster.
type ClusterTransport struct {
	clients *Clients
	shared.BaseTransport
}

func newClusterTransport(clients *Clients) *ClusterTransport {
	return &ClusterTransport{
		clients: clients,
	}
}

// 2-AKE messages are sent to specific clients, Xi and Key messages are broadcasted.
func (t *ClusterTransport) Send(msg shared.Message) {
	switch msg.Type {
	case shared.AkeAMsg, shared.AkeBMsg:
		t.clients.send(msg)
	case shared.KeyMsg, shared.XiRiCommitmentMsg:
		t.clients.broadcast(msg)
	case shared.TextMsg:
		t.clients.broadcast(msg)
		broadcastToLeaders(msg)
	}
}

func (t *ClusterTransport) receive(msg shared.Message) {
	t.MessageHandler(msg)
}

// Leader Transport for communication between leaders.
type LeaderTransport struct {
	shared.BaseTransport
}

func newLeaderTransport() *LeaderTransport {
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

func (t *LeaderTransport) receive(msg shared.Message) {
	t.MessageHandler(msg)
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
	fmt.Printf("[ROUTE] Sending message %s to Leader %s\n", msg.TypeName(), address)

	msg.Send(conn)
}
