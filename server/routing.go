package main

import (
	"fmt"
	"pqgch-client/shared"
)

func sendMsgToClient(msg shared.Message) {
	muClients.Lock()
	defer muClients.Unlock()

	for client := range clients {
		if client.index == msg.ReceiverID && client.index != msg.SenderID {
			err := shared.SendMsg(client.conn, msg)
			if err != nil {
				fmt.Println("Error sending message to client:", err)
				client.conn.Close()
				delete(clients, client)
			}

			fmt.Printf("Sent message to %s\n", client.name)
			return
		}

	}
	fmt.Printf("Not sending message: either did not find client, or sender is receiver\n")
}

func broadcastMessage(msg shared.Message) {
	muClients.Lock()
	defer muClients.Unlock()

	for client := range clients {
		if client.index == msg.SenderID {
			continue
		}

		err := shared.SendMsg(client.conn, msg)
		if err != nil {
			fmt.Println("Error sending message to client:", err)
			client.conn.Close()
			delete(clients, client)
			return
		}
	}

	fmt.Printf("Broadcasted message from %s\n", msg.SenderName)
}

func forwardMessage(msg shared.Message) {
	muNeighborConn.Lock()
	defer muNeighborConn.Unlock()

	if neighborConn == nil {
		fmt.Println("No connection to left neighbor; message not forwarded.")
		return
	}

	err := shared.SendMsg(neighborConn, msg)
	if err != nil {
		fmt.Println("Error forwarding message to left neighbor:", err)
		neighborConn.Close()
		neighborConn = nil
		return
	}

	fmt.Printf("Forwarded message from %s: %s\n", msg.SenderName, msg.Content)
}
