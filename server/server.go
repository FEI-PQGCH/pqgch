package main

import (
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
	clusterSession     shared.Session
	mainSession        shared.Session
)

func initialize() net.Listener {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	if *configFlag != "" {
		config = shared.GetServConfig(*configFlag)
	} else {
		fmt.Println("please provide a configuration file using the -config flag.")
		panic("no configuration file provided")
	}

	_, selfPort, err := net.SplitHostPort(config.GetCurrentServer())
	if err != nil {
		fmt.Println("error parsing self address from config:", err)
		panic("error parsing self address from config")
	}
	port := selfPort

	address := fmt.Sprintf(":%s", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("error starting TCP server:", err)
		panic("error starting TCP server")
	}

	fmt.Println("server listening on", address)

	clusterSession = shared.MakeSession(&config.ClusterConfig)
	mainSession = shared.MakeSession(&config)

	return listener
}

func main() {
	listener := initialize()
	defer listener.Close()

	go connectNeighbor(config.GetRightNeighbor())

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("error accepting connection:", err)
			continue
		}

		go clientLogin(conn)
	}
}

func connectNeighbor(neighborAddress string) {
	for {
		muNeighborConn.Lock()
		fmt.Printf("connecting to right neighbor at %s\n", neighborAddress)
		conn, err := net.Dial("tcp", neighborAddress)
		if err != nil {
			fmt.Printf("error connecting to right neighbor: %v. Retrying...\n", err)
			muNeighborConn.Unlock()
			time.Sleep(2 * time.Second)
			continue
		}
		neighborConn = conn
		fmt.Printf("connected to right neighbor (%s)\n", neighborAddress)
		loginMsg := shared.Message{
			ID:         uuid.New().String(),
			SenderID:   -1,
			SenderName: "server",
			Type:       shared.LoginMsg,
		}

		loginMsg.Send(neighborConn)
		msg := shared.GetAkeAMsg(&mainSession, &config)
		msg.Send(neighborConn)
		fmt.Println("CRYPTO: sending Leader AKE A message")

		muNeighborConn.Unlock()
		break
	}

	msgReader := shared.NewMessageReader(neighborConn)

	if !msgReader.HasMessage() {
		fmt.Println("right neighbor did not send login message")
		neighborConn.Close()
		muNeighborConn.Lock()
		neighborConn = nil
		muNeighborConn.Unlock()
		return
	}

	msg := msgReader.GetMessage()
	fmt.Printf("RECEIVED: %s from %s \n", msg.MsgTypeName(), msg.SenderName)
	handler := GetHandler(msg)
	handler.HandleMessage(nil, msg)
}

func clientLogin(conn net.Conn) {
	fmt.Println("new client connected:", conn.RemoteAddr())

	msgReader := shared.NewMessageReader(conn)
	if !msgReader.HasMessage() {
		fmt.Println("client did not send login message")
		conn.Close()
		return
	}

	msg := msgReader.GetMessage()
	if msg.Type != shared.LoginMsg {
		fmt.Println("client did not send login message")
		conn.Close()
		return
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
		msg := shared.GetAkeAMsg(&clusterSession, &config.ClusterConfig)
		sendMsgToClient(msg)
	}

	handleClient(client, *msgReader)
}

func handleClient(client Client, reader shared.MessageReader) {
	defer func() {
		muClients.Lock()
		delete(clients, client)
		muClients.Unlock()
		client.conn.Close()
		fmt.Println("client disconnected:", client.conn.RemoteAddr())
	}()

	for reader.HasMessage() {
		msg := reader.GetMessage()
		fmt.Printf("RECEIVED: %s from %s \n", msg.MsgTypeName(), msg.SenderName)

		muReceivedMessages.Lock()
		if receivedMessages[msg.ID] {
			muReceivedMessages.Unlock()
			continue
		}
		receivedMessages[msg.ID] = true
		muReceivedMessages.Unlock()

		if client.index != -1 {
			msg.ClusterID = config.Index
		}

		handler := GetHandler(msg)
		handler.HandleMessage(client.conn, msg)
	}
}
