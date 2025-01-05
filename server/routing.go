package main

import (
	"fmt"
	"pqgch-client/shared"
	"time"
)

// send a message to a client in this cluster
func sendToClient(msg shared.Message) {
	muClients.Lock()
	defer muClients.Unlock()

	for client := range clients {
		if client.index == msg.ReceiverID && client.index != msg.SenderID {
			err := msg.Send(client.conn)
			if err != nil {
				fmt.Println("error sending message to client:", err)
				client.conn.Close()
				delete(clients, client)
			}

			fmt.Printf("ROUTE: sent message %s to %s\n", msg.TypeName(), client.name)
			return
		}

	}
	fmt.Printf("error: sending message: either did not find client, or sender is receiver\n")
}

// broadcast a message to all clients in this cluster except the sender
func broadcastToCluster(msg shared.Message) {
	muClients.Lock()
	defer muClients.Unlock()

	for client := range clients {
		if client.index == msg.SenderID && msg.ClusterID == config.Index {
			continue
		}

		err := msg.Send(client.conn)
		if err != nil {
			fmt.Println("error sending message to client:", err)
			client.conn.Close()
			delete(clients, client)
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
