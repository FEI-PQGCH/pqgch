package main

import (
	"fmt"
	"net"
	"os"
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
func (clients *Clients) send(msg util.Message) {
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
			fmt.Fprintf(os.Stderr, "[ERROR] sending message to client: %v\n", err)
			c.conn.Close()
			return
		}
	}
	fmt.Printf("[ROUTE] Broadcasted message %s from %s\n", msg.TypeName(), msg.SenderName)
}

type ServerTransport interface {
	receive(util.Message)
}

func transportManager(t ServerTransport, msgs <-chan util.Message) {
	for msg := range msgs {
		t.receive(msg)
	}
}

// Cluster transport for communication between the leader and clients in its cluster.
type ClusterTransport struct {
	clients *Clients
	util.BaseTransport
}

func newClusterTransport(clients *Clients) *ClusterTransport {
	return &ClusterTransport{
		clients: clients,
	}
}

// 2-AKE messages are sent to specific clients, Xi and Key messages are broadcasted.
func (t *ClusterTransport) Send(msg util.Message) {
	switch msg.Type {
	case util.AkeAMsg, util.AkeBMsg:
		t.clients.send(msg)
	case util.KeyMsg, util.XiRiCommitmentMsg:
		t.clients.broadcast(msg)
	case util.TextMsg:
		t.clients.broadcast(msg)
		broadcastToLeaders(msg)
	}
}

func (t *ClusterTransport) receive(msg util.Message) {
	t.MessageHandler(msg)
}

// Leader Transport for communication between leaders.
type LeaderTransport struct {
	util.BaseTransport
}

func newLeaderTransport() *LeaderTransport {
	return &LeaderTransport{}
}

// 2-AKE messages are sent only to specific leaders, the Xi message is broadcasted.
func (t *LeaderTransport) Send(msg util.Message) {
	switch msg.Type {
	case util.LeaderAkeAMsg:
		sendToLeader(config.Addrs[msg.ReceiverID], msg)
	case util.LeaderAkeBMsg:
		sendToLeader(config.Addrs[msg.ReceiverID], msg)
	case util.LeaderXiRiCommitmentMsg:
		broadcastToLeaders(msg)
	}
}

func (t *LeaderTransport) receive(msg util.Message) {
	t.MessageHandler(msg)
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
			fmt.Fprintf(os.Stderr, "[ERROR] Leader (%s) connection error: %v. Retrying...\n", address, err)
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	fmt.Printf("[ROUTE] Sending message %s to Leader %s\n", msg.TypeName(), address)

	msg.Send(conn)
}

// TODO: sort out the SAE IDs
func requestKey(leaderChan chan<- util.Message, url string) {
	key, keyID, err := util.GetKey(url, "dummy_id")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// Process the received key.
	msg := util.Message{
		ID:         util.UniqueID(),
		SenderID:   -1,
		SenderName: "ETSI API",
		Type:       util.QKDRightKeyMsg,
		ReceiverID: config.Index,
		Content:    key,
	}
	leaderChan <- msg

	// Send keyID to right neighbor.
	receiverID := (config.Index + 1) % len(config.Addrs)
	msg = util.Message{
		ID:         util.UniqueID(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Type:       util.QKDIDsMsg,
		ReceiverID: receiverID,
		Content:    keyID,
	}
	sendToLeader(config.Addrs[receiverID], msg)
}

func requestKeyWithID(leaderChan chan<- util.Message, url, id string) {
	key, _, err := util.GetKeyWithID(url, "dummy_id", id)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// Process the received key.
	msg := util.Message{
		ID:         util.UniqueID(),
		SenderID:   -1,
		SenderName: "ETSI API",
		Type:       util.QKDLeftKeyMsg,
		ReceiverID: config.Index,
		Content:    key,
	}
	leaderChan <- msg
}
