package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"pqgch-client/shared"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	name  string
	conn  net.Conn
	index int
}

var (
	receivedMessages   = make(map[string]bool)
	muReceivedMessages sync.Mutex
	clients            = make(map[Client]bool)
	muClients          sync.Mutex
	neighborConn       net.Conn
	muNeighborConn     sync.Mutex
	config             shared.ServConfig
)

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	if *configFlag != "" {
		config = shared.GetServConfig(*configFlag)
	} else {
		fmt.Println("Please provide a configuration file using the -config flag.")
		return
	}

	_, selfPort, err := net.SplitHostPort(config.GetCurrentServer())
	if err != nil {
		fmt.Println("Error parsing self address from config:", err)
		return
	}
	port := selfPort

	address := fmt.Sprintf(":%s", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("Error starting TCP server:", err)
		return
	}
	defer listener.Close()
	fmt.Println("Server listening on", address)

	go connectNeighbor(config.GetLeftNeighbor())

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		clientLogin(conn)
	}
}

func clientLogin(conn net.Conn) {
	fmt.Println("New client connected:", conn.RemoteAddr())

	scanner := bufio.NewScanner(conn)
	scanner.Scan()
	msgData := scanner.Bytes()

	var msg shared.Message
	err := json.Unmarshal(msgData, &msg)
	if err != nil {
		fmt.Println("Error unmarshaling message:", err)
		conn.Close()
	}

	if msg.MsgType != shared.MsgLogin {
		fmt.Println("Client did not send login message")
		conn.Close()
	}

	client := Client{name: msg.SenderName, conn: conn, index: msg.SenderID}

	muClients.Lock()
	clients[client] = true
	muClients.Unlock()

	go handleConnection(client)
}

func connectNeighbor(neighborAddress string) {
	for {
		muNeighborConn.Lock()
		if neighborConn == nil {
			fmt.Printf("Connecting to left neighbor at %s\n", neighborAddress)
			conn, err := net.Dial("tcp", neighborAddress)
			if err != nil {
				fmt.Printf("Error connecting to left neighbor: %v. Retrying...\n", err)
				muNeighborConn.Unlock()
				time.Sleep(2 * time.Second)
				continue
			}
			neighborConn = conn
			fmt.Printf("Connected to left neighbor (%s)\n", neighborAddress)
			loginMsg := shared.Message{
				MsgID:      uuid.New().String(),
				SenderID:   -1,
				SenderName: "server",
				MsgType:    shared.MsgLogin,
			}

			msgData, err := json.Marshal(loginMsg)
			if err != nil {
				fmt.Println("Error marshaling message:", err)
				muNeighborConn.Unlock()
				continue
			}

			_, err = neighborConn.Write(append(msgData, '\n'))
			if err != nil {
				fmt.Printf("Error sending login message to left neighbor: %v\n", err)
				neighborConn.Close()
				neighborConn = nil
			}
		}
		muNeighborConn.Unlock()
		time.Sleep(5 * time.Second)
	}
}

func handleConnection(client Client) {
	defer func() {
		muClients.Lock()
		delete(clients, client)
		muClients.Unlock()
		client.conn.Close()
		fmt.Println("Client disconnected:", client.conn.RemoteAddr())
	}()

	scanner := bufio.NewScanner(client.conn)
	for scanner.Scan() {
		msgData := scanner.Bytes()

		var msg shared.Message
		err := json.Unmarshal(msgData, &msg)
		if err != nil {
			fmt.Println("Error unmarshaling message:", err)
			continue
		}

		muReceivedMessages.Lock()
		if receivedMessages[msg.MsgID] {
			muReceivedMessages.Unlock()
			continue
		}
		receivedMessages[msg.MsgID] = true
		muReceivedMessages.Unlock()

		fmt.Printf("Received message from %s: %s\n", msg.SenderName, msg.Content)

		if msg.MsgType == shared.MsgBroadcast {
			broadcastMessage(msg, client)
			forwardMessage(msg)
		}

		if msg.MsgType == shared.MsgIntra {
			sendMsgToClient(msg)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from client:", err)
	}
}

func sendMsgToClient(msg shared.Message) {
	muClients.Lock()
	defer muClients.Unlock()

	for client := range clients {
		if client.index == msg.ReceiverID && client.index != msg.SenderID {
			msgData, err := json.Marshal(msg)
			if err != nil {
				fmt.Println("Error marshaling message:", err)
				continue
			}

			_, err = client.conn.Write(append(msgData, '\n'))
			if err != nil {
				fmt.Println("Error sending message to client:", err)
				client.conn.Close()
				delete(clients, client)
			}
			fmt.Printf("Sent message to %s: %s\n", client.name, msg.Content)
			return
		}
	}
	fmt.Printf("Not sending message: either did not find client, or sender is receiver\n")
}

func broadcastMessage(msg shared.Message, sender Client) {
	muClients.Lock()
	defer muClients.Unlock()

	for client := range clients {
		if client == sender {
			continue
		}

		msgData, err := json.Marshal(msg)
		if err != nil {
			fmt.Println("Error marshaling message:", err)
			continue
		}

		_, err = client.conn.Write(append(msgData, '\n'))
		if err != nil {
			fmt.Println("Error sending message to client:", err)
			client.conn.Close()
			delete(clients, client)
		}
	}
	fmt.Printf("Broadcasted message from %s: %s\n", msg.SenderName, msg.Content)
}

func forwardMessage(msg shared.Message) {
	muNeighborConn.Lock()
	defer muNeighborConn.Unlock()

	if neighborConn == nil {
		fmt.Println("No connection to left neighbor; message not forwarded.")
		return
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		fmt.Println("Error marshaling message:", err)
		return
	}

	msgData = append(msgData, '\n')

	_, err = neighborConn.Write(msgData)
	if err != nil {
		fmt.Printf("Error forwarding message to left neighbor: %v\n", err)
		neighborConn.Close()
		neighborConn = nil
	} else {
		fmt.Printf("Message forwarded to left neighbor\n")
	}
}
