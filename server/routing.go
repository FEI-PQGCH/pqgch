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
				fmt.Println("error sending message to client:", err)
				client.conn.Close()
				delete(clients, client)
			}

			fmt.Printf("ROUTE: sent message %s to %s\n", msg.MsgTypeName(), client.name)
			return
		}

	}
	fmt.Printf("error: sending message: either did not find client, or sender is receiver\n")
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
			fmt.Println("error sending message to client:", err)
			client.conn.Close()
			delete(clients, client)
			return
		}
	}

	fmt.Printf("ROUTE: broadcasted message %s from %s\n", msg.MsgTypeName(), msg.SenderName)
}

func forwardMessage(msg shared.Message) {
	muNeighborConn.Lock()
	defer muNeighborConn.Unlock()

	if neighborConn == nil {
		fmt.Println("no connection to left neighbor; message not forwarded.")
		return
	}

	err := shared.SendMsg(neighborConn, msg)
	if err != nil {
		fmt.Println("error forwarding message to left neighbor:", err)
		neighborConn.Close()
		neighborConn = nil
		return
	}

	fmt.Printf("ROUTE: forwarded message %s from %s\n", msg.MsgTypeName(), msg.SenderName)
}
