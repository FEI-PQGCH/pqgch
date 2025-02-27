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
	queues         []MessageQueue
	clients        []Client
	muClients      sync.Mutex
	neighborConn   net.Conn
	muNeighborConn sync.Mutex
	config         shared.ServConfig
	clusterSession shared.Session
	mainSession    shared.Session
)

type MessageQueue []shared.Message

func (mq *MessageQueue) add(msg shared.Message) {
	*mq = append(*mq, msg)
}

func (mq *MessageQueue) remove(msg shared.Message) {
	tmp := MessageQueue{}
	for _, x := range *mq {
		if x.ID != msg.ID {
			tmp.add(x)
		}
	}
	*mq = tmp
}

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	if *configFlag != "" {
		config = shared.GetServConfig(*configFlag)
	} else {
		fmt.Printf("[ERROR] Configuration file missing. Please provide it using the -config flag.\n")
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
	defer listener.Close()
	fmt.Println("server listening on", address)

	clusterSession = shared.MakeSession(&config.ClusterConfig)
	clusterSession.OnSharedKey = onClusterSession

	mainSession = shared.MakeSession(&config)
	tracker := shared.NewMessageTracker()

	queues = make([]MessageQueue, len(config.ClusterConfig.GetNamesOrAddrs()))

	for i := range len(config.ClusterConfig.GetNamesOrAddrs()) {
		if i == config.ClusterConfig.GetIndex() {
			continue
		}

		client := Client{name: config.ClusterConfig.GetNamesOrAddrs()[i], conn: nil, index: i}
		clients = append(clients, client)
	}

	go connectNeighbor(config.GetRightNeighbor())

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("error accepting connection:", err)
			continue
		}

		go handleClient(conn, tracker)
	}
}

func connectNeighbor(neighborAddress string) {
	for {
		muNeighborConn.Lock()
		fmt.Printf("[INFO] Attempting connection to right neighbor at %s\n", neighborAddress)
		conn, err := net.Dial("tcp", neighborAddress)
		if err != nil {
			fmt.Printf("[ERROR] Right neighbor connection error: %v. Retrying...\n", err)
			muNeighborConn.Unlock()
			time.Sleep(2 * time.Second)
			continue
		}
		neighborConn = conn
		fmt.Printf("[INFO] Connected to right neighbor (%s)\n", neighborAddress)
		loginMsg := shared.Message{
			ID:         uuid.New().String(),
			SenderID:   -1,
			SenderName: "server",
			Type:       shared.LoginMsg,
		}

		loginMsg.Send(neighborConn)
		msg := shared.GetAkeAMsg(&mainSession, &config)
		msg.Send(neighborConn)
		fmt.Printf("[CRYPTO] Sending Leader AKE A message\n")

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
	fmt.Printf("[INFO] Received %s from %s \n", msg.TypeName(), msg.SenderName)
	handleMessage(nil, msg)
}

func handleClient(conn net.Conn, tracker *shared.MessageTracker) {
	fmt.Println("[INFO] New client connected:", conn.RemoteAddr())

	reader := shared.NewMessageReader(conn)
	if !reader.HasMessage() {
		fmt.Println("error: client did not send message")
		conn.Close()
		return
	}

	msg := reader.GetMessage()
	if msg.Type != shared.LoginMsg {
		fmt.Println("error: client did not send login message")
		conn.Close()
		return
	}

	client := Client{name: msg.SenderName, conn: conn, index: msg.SenderID}

	if msg.SenderID != -1 {
		muClients.Lock()
		clients[msg.SenderID].conn = conn
		muClients.Unlock()
		defer func() {
			/* TODO: handle disconnect */
			client.conn.Close()
			fmt.Println("client disconnected:", client.conn.RemoteAddr())
		}()

		for _, msg := range queues[client.index] {
			msg.Send(client.conn)
			queues[client.index].remove(msg)
		}
	}

	clusterClientCount := 0
	for i := range clients {
		if clients[i].conn != nil {
			clusterClientCount++
		}
	}

	if clusterClientCount == len(config.Names)-1 {
		msg := shared.GetAkeAMsg(&clusterSession, &config.ClusterConfig)
		sendToClient(msg)
	}

	for reader.HasMessage() {
		msg := reader.GetMessage()
		fmt.Printf("[INFO] Received %s from %s \n", msg.TypeName(), msg.SenderName)

		if !tracker.AddMessage(msg.ID) {
			continue
		}

		if client.index != -1 {
			msg.ClusterID = config.Index
		}

		handleMessage(client.conn, msg)
	}
}
