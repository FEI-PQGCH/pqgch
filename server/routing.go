package main

import (
	"fmt"
	"net"
	"pqgch-client/shared"
	"time"
)

type ClusterTransport struct {
	shared.BaseTransport
}

func NewClusterTransport() *ClusterTransport {
	return &ClusterTransport{}
}

func (t *ClusterTransport) Send(msg shared.Message) {
	switch msg.Type {
	case shared.XiMsg:
		broadcastToCluster(msg)
	case shared.AkeAMsg:
		sendToClient(msg)
	case shared.AkeBMsg:
		sendToClient(msg)
	case shared.KeyMsg:
		broadcastToCluster(msg)
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

func sendToClient(msg shared.Message) {
	client := clients[msg.ReceiverID]

	if client.conn == nil {
		queues[msg.ReceiverID].add(msg)
		fmt.Printf("[DEBUG] Route: stored message\n")
		return
	}

	msg.Send(client.conn)
}

func broadcastToCluster(msg shared.Message) {
	muClients.Lock()
	defer muClients.Unlock()

	for i := range clients {
		client := clients[i]

		if client.index == msg.SenderID && msg.ClusterID == config.Index {
			continue
		}

		if client.conn == nil {
			queues[i].add(msg)
			continue
		}

		err := msg.Send(client.conn)
		if err != nil {
			fmt.Println("error sending message to client:", err)
			client.conn.Close()
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
