package main

import (
	"flag"
	"fmt"
	"net"
	"pqgch-client/shared"
	"sync"
)

type Client struct {
	name  string
	conn  net.Conn
	index int
}

var (
	queues           []MessageQueue
	clients          []Client
	muClients        sync.Mutex
	config           shared.ServConfig
	devSession       *shared.DevSession
	mainDevSession   *shared.LeaderDevSession
	clusterTransport *ClusterTransport
	leaderTransport  *LeaderTransport
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

	fmt.Println("[CRYPTO] Using GAKE handshake to derive master key")

	_, selfPort, err := net.SplitHostPort(config.GetCurrentServer())
	if err != nil {
		fmt.Println("[ERROR] Error parsing self address from config:", err)
		panic("error parsing self address from config")
	}
	port := selfPort
	address := fmt.Sprintf(":%s", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("[ERROR] Error starting TCP server:", err)
		panic("error starting TCP server")
	}
	defer listener.Close()
	fmt.Println("server listening on", address)

	tracker := shared.NewMessageTracker()

	leaderTransport = NewLeaderTransport()
	mainDevSession = shared.NewLeaderDevSession(leaderTransport, &config)

	clusterTransport = NewClusterTransport()
	devSession = shared.NewClusterLeaderSession(clusterTransport, &config.ClusterConfig, mainDevSession.GetKeyRef())

	queues = make([]MessageQueue, len(config.ClusterConfig.GetNamesOrAddrs()))
	for i := range config.ClusterConfig.GetNamesOrAddrs() {
		if i == config.ClusterConfig.GetIndex() {
			continue
		}
		client := Client{
			name:  config.ClusterConfig.GetNamesOrAddrs()[i],
			conn:  nil,
			index: i,
		}
		clients = append(clients, client)
	}

	mainDevSession.Init()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("[ERROR] Error accepting connection:", err)
			continue
		}
		go handleConnection(conn, tracker)
	}
}

func handleConnection(conn net.Conn, tracker *shared.MessageTracker) {
	reader := shared.NewMessageReader(conn)
	if !reader.HasMessage() {
		fmt.Println("[ERROR] Client did not send any message")
		conn.Close()
		return
	}

	msg := reader.GetMessage()
	if msg.Type != shared.LoginMsg {
		if !tracker.AddMessage(msg.ID) {
			return
		}
		fmt.Printf("[INFO] Received %s message from Leader\n", msg.TypeName())
		if msg.Type == shared.BroadcastMsg {
			broadcastToCluster(msg)
			return
		}
		leaderTransport.Receive(msg)
		conn.Close()
		return
	}
	fmt.Printf("[INFO] New client (%s, %s) joined", msg.SenderName, conn.RemoteAddr())

	client := Client{
		name:  msg.SenderName,
		conn:  conn,
		index: msg.SenderID,
	}

	if msg.SenderID != -1 {
		muClients.Lock()
		clients[msg.SenderID].conn = conn
		muClients.Unlock()
		defer func() {
			client.conn.Close()
			fmt.Println("[INFO] Client disconnected:", client.conn.RemoteAddr())
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
		devSession.Init()
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

		if msg.ReceiverID == config.ClusterConfig.Index && msg.Type == shared.AkeAMsg {
			clusterTransport.Receive(msg)
		}
		if msg.ReceiverID == config.ClusterConfig.Index && msg.Type == shared.AkeBMsg {
			clusterTransport.Receive(msg)
		}
		if msg.ReceiverID != config.ClusterConfig.Index && msg.Type == shared.AkeAMsg {
			sendToClient(msg)
		}
		if msg.ReceiverID != config.ClusterConfig.Index && msg.Type == shared.AkeBMsg {
			sendToClient(msg)
		}
		if msg.Type == shared.XiMsg {
			clusterTransport.Receive(msg)
			broadcastToCluster(msg)
		}
		if msg.Type == shared.BroadcastMsg {
			broadcastToCluster(msg)
			broadcastToLeaders(msg)
		}
		if msg.Type == shared.LeaderAkeAMsg || msg.Type == shared.LeaderAkeBMsg || msg.Type == shared.LeaderXiMsg {
			leaderTransport.Receive(msg)
		}
	}
}
