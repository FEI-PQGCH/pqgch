package main

import (
	"fmt"
	"pqgch-client/shared"
	"sync"
	"time"
)

type ServerTransport struct {
	messageHandler func(shared.Message)
	mu             sync.Mutex
}

func NewServerTransport() *ServerTransport {
	t := &ServerTransport{}
	return t
}

func (t *ServerTransport) SetMessageHandler(handler func(shared.Message)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.messageHandler = handler
}

func (t *ServerTransport) Send(msg shared.Message) {
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

func (t *ServerTransport) Receive(msg shared.Message) {
	t.messageHandler(msg)
}

// send a message to a client in this cluster
func sendToClient(msg shared.Message) {
	client := clients[msg.ReceiverID]

	if client.conn == nil {
		queues[msg.ReceiverID].add(msg)
		fmt.Printf("[DEBUG] Route: stored message\n")
		return
	}

	msg.Send(client.conn)
}

// broadcast a message to all clients in this cluster except the sender
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
			/* TODO: handle error */
			return
		}
	}
	fmt.Printf("ROUTE: broadcasted message %s from %s\n", msg.TypeName(), msg.SenderName)
}

// forward a message to the right neighbor.
func forwardToNeighbor(msg shared.Message) {
	muNeighborConn.Lock()
	defer muNeighborConn.Unlock()

	for {
		if neighborConn == nil {
			fmt.Println("no connection to right neighbor; waiting.")
			muNeighborConn.Unlock()
			time.Sleep(1 * time.Second)
			muNeighborConn.Lock()
			continue
		}

		err := msg.Send(neighborConn)
		if err != nil {
			fmt.Println("error forwarding message to right neighbor:", err)
			neighborConn.Close()
			neighborConn = nil
			return
		}
		fmt.Printf("ROUTE: forwarded message %s from %s\n", msg.TypeName(), msg.SenderName)
		break
	}
}

func broadcast(msg shared.Message) {
	broadcastToCluster(msg)
	forwardToNeighbor(msg)
}
