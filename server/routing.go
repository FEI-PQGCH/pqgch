package main

import (
	"fmt"
	"net"
	"pqgch/util"
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
	queue util.MessageQueue
}

func newClients(config util.ClusterConfig) *Clients {
	var clients Clients

	for i, addr := range config.Names {
		if i == config.Index {
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
	util.LogInfo(fmt.Sprint("Client disconnected:", clients.data[id].conn.RemoteAddr()))
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
func (clients *Clients) send(msg util.Message) {
	clients.lock.Lock()
	defer clients.lock.Unlock()

	client := &clients.data[msg.ReceiverID]
	if client.conn == nil {
		client.queue.Add(msg)
		util.LogRoute("Stored message")
		return
	}
	msg.Send(client.conn)
}

// Broadcast message to all clients
// If some of the clients are not available, store their messages in their queues.
func (clients *Clients) broadcast(msg util.Message) {
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
			util.LogError(fmt.Sprintf("Sending message to client: %v", err))
			c.conn.Close()
			return
		}
	}
	util.LogRoute(fmt.Sprintf("Broadcasted %s from %s", msg.TypeName(), msg.SenderName))
}

// Cluster transport for communication between the leader and clients in its cluster.
type ClusterMessageSender struct {
	clients *Clients
}

func newClusterMessageSender(clients *Clients) *ClusterMessageSender {
	return &ClusterMessageSender{
		clients: clients,
	}
}

// 2-AKE messages are sent to specific clients, Xi and Key messages are broadcasted.
func (t *ClusterMessageSender) Send(msg util.Message) {
	switch msg.Type {
	case util.AkeOneMsg, util.AkeTwoMsg:
		t.clients.send(msg)
	case util.KeyMsg, util.XiRiCommitmentMsg, util.QKDIDMsg:
		t.clients.broadcast(msg)
	case util.TextMsg:
		msg.ClusterID = config.Index
		t.clients.broadcast(msg)
		broadcastToLeaders(msg)
	}
}

// Leader Transport for communication between leaders.
type LeaderMessageSender struct{}

func newLeaderMessageSender() *LeaderMessageSender {
	return &LeaderMessageSender{}
}

// 2-AKE messages are sent only to specific leaders, the Xi message is broadcasted.
func (t *LeaderMessageSender) Send(msg util.Message) {
	switch msg.Type {
	case util.LeadAkeOneMsg, util.LeadAkeTwoMsg:
		sendToLeader(config.Addrs[msg.ReceiverID], msg)
	case util.LeaderXiRiCommitmentMsg:
		broadcastToLeaders(msg)
	}
}

// Broadcast msg to all the other leaders.
func broadcastToLeaders(msg util.Message) {
	for i, addr := range config.Addrs {
		if i == config.Index {
			continue
		}
		sendToLeader(addr, msg)
	}
}

// Send msg to specific leader at address.
func sendToLeader(address string, msg util.Message) {
	var conn net.Conn
	for {
		var err error
		conn, err = net.Dial("tcp", address)
		if err != nil {
			util.LogError(fmt.Sprintf("Leader (%s) connection error: %v. Retrying...", address, err))
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	msg.Send(conn)
	util.LogRoute(fmt.Sprintf("Sent %s to Leader %s", msg.TypeName(), address))
}
