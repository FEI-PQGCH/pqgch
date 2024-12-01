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
	session            shared.Session
)

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	if *configFlag != "" {
		config = shared.GetServConfig(*configFlag)
	} else {
		fmt.Println("please provide a configuration file using the -config flag.")
		return
	}

	_, selfPort, err := net.SplitHostPort(config.GetCurrentServer())
	if err != nil {
		fmt.Println("error parsing self address from config:", err)
		return
	}
	port := selfPort

	address := fmt.Sprintf(":%s", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("error starting TCP server:", err)
		return
	}
	defer listener.Close()
	fmt.Println("server listening on", address)
	session.Xs = make([][32]byte, len(config.Names))

	go connectNeighbor(config.GetLeftNeighbor())

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("error accepting connection:", err)
			continue
		}

		clientLogin(conn)
	}
}

func clientLogin(conn net.Conn) {
	fmt.Println("new client connected:", conn.RemoteAddr())

	scanner := bufio.NewScanner(conn)
	scanner.Scan()
	msgData := scanner.Bytes()

	var msg shared.Message
	err := json.Unmarshal(msgData, &msg)
	if err != nil {
		fmt.Println("error unmarshaling message:", err)
		conn.Close()
	}

	if msg.MsgType != shared.LoginMsg {
		fmt.Println("client did not send login message")
		conn.Close()
	}

	client := Client{name: msg.SenderName, conn: conn, index: msg.SenderID}

	muClients.Lock()
	clients[client] = true
	muClients.Unlock()

	clusterClientCount := 0
	for c := range clients {
		if c.index != -1 {
			clusterClientCount++
		}
	}

	if clusterClientCount == len(config.Names)-1 {
		msg := shared.GetAkeInitAMsg(&session, config.ClusterConfig)
		sendMsgToClient(msg)
	}

	go handleConnection(client)
}

func connectNeighbor(neighborAddress string) {
	for {
		muNeighborConn.Lock()
		if neighborConn == nil {
			fmt.Printf("connecting to left neighbor at %s\n", neighborAddress)
			conn, err := net.Dial("tcp", neighborAddress)
			if err != nil {
				fmt.Printf("error connecting to left neighbor: %v. Retrying...\n", err)
				muNeighborConn.Unlock()
				time.Sleep(2 * time.Second)
				continue
			}
			neighborConn = conn
			fmt.Printf("connected to left neighbor (%s)\n", neighborAddress)
			loginMsg := shared.Message{
				MsgID:      uuid.New().String(),
				SenderID:   -1,
				SenderName: "server",
				MsgType:    shared.LoginMsg,
			}

			err = shared.SendMsg(neighborConn, loginMsg)
			if err != nil {
				fmt.Printf("error sending login message to left neighbor: %v\n", err)
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
		fmt.Println("client disconnected:", client.conn.RemoteAddr())
	}()

	scanner := bufio.NewScanner(client.conn)
	for scanner.Scan() {
		msgData := scanner.Bytes()

		var msg shared.Message
		err := json.Unmarshal(msgData, &msg)
		if err != nil {
			fmt.Println("error unmarshaling message:", err)
			continue
		}

		muReceivedMessages.Lock()
		if receivedMessages[msg.MsgID] {
			muReceivedMessages.Unlock()
			continue
		}
		receivedMessages[msg.MsgID] = true
		muReceivedMessages.Unlock()

		if client.index != -1 {
			msg.ClusterID = config.Index
		}

		fmt.Printf("RECEIVED: %s from %s\n", msg.MsgTypeName(), msg.SenderName)

		handler := GetHandler(msg)
		handler.HandleMessage(msg)
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("error reading from client:", err)
	}
}
