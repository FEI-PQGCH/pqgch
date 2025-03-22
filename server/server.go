package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"pqgch/cluster_protocol"
	"pqgch/leader_protocol"
	"pqgch/shared"
)

var (
	config shared.ServConfig
)

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	if *configFlag != "" {
		config = shared.GetServConfig(*configFlag)
	} else {
		fmt.Printf("[ERROR] Configuration file missing. Please provide it using the -config flag.\n")
		os.Exit(1)
	}

	fmt.Println("[CRYPTO] Using GAKE handshake to derive master key")

	_, selfPort, err := net.SplitHostPort(config.GetCurrentServer())
	if err != nil {
		fmt.Println("[ERROR] Error parsing self address from config:", err)
		os.Exit(1)
	}
	port := selfPort
	address := fmt.Sprintf(":%s", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("[ERROR] Error starting TCP server:", err)
		os.Exit(1)
	}
	defer listener.Close()
	fmt.Println("server listening on", address)

	tracker := shared.NewMessageTracker()

	leaderTransport := NewLeaderTransport()
	leaderSession := leader_protocol.NewLeaderSession(leaderTransport, &config)

	var clients Clients
	for i, addr := range config.ClusterConfig.GetNamesOrAddrs() {
		if i == config.ClusterConfig.GetIndex() {
			continue
		}
		client := Client{
			name:  addr,
			conn:  nil,
			index: i,
		}
		clients.cs = append(clients.cs, client)
	}

	clusterTransport := NewClusterTransport(&clients)
	clusterSession := cluster_protocol.NewClusterLeaderSession(clusterTransport, &config.ClusterConfig, leaderSession.GetKeyRef())

	leaderSession.Init()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("[ERROR] Error accepting connection:", err)
			continue
		}
		go handleConnection(&clients, conn, tracker, clusterSession, clusterTransport, leaderTransport)
	}
}

func handleConnection(
	clients *Clients,
	conn net.Conn,
	tracker *shared.MessageTracker,
	session *cluster_protocol.ClusterSession,
	clusterTransport *ClusterTransport,
	leaderTransport *LeaderTransport) {
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
		if msg.Type == shared.TextMsg {
			broadcastToCluster(msg, clients)
			return
		}
		leaderTransport.Receive(msg)
		conn.Close()
		return
	}
	fmt.Printf("[INFO] New client (%s, %s) joined\n", msg.SenderName, conn.RemoteAddr())
	clientID := msg.SenderID

	clients.mu.Lock()
	clients.cs[clientID].conn = conn
	clients.mu.Unlock()
	defer func() {
		clients.mu.Lock()
		clients.cs[clientID].conn.Close()
		clients.mu.Unlock()
		fmt.Println("[INFO] Client disconnected:", clients.cs[clientID].conn.RemoteAddr())
	}()

	for _, msg := range clients.cs[clientID].queue {
		msg.Send(clients.cs[clientID].conn)
		clients.mu.Lock()
		clients.cs[clientID].queue.remove(msg)
		clients.mu.Unlock()
	}

	onlineClients := 0
	for _, c := range clients.cs {
		if c.conn != nil {
			onlineClients++
		}
	}
	if onlineClients == len(config.Names)-1 {
		session.Init()
	}

	for reader.HasMessage() {
		msg := reader.GetMessage()
		fmt.Printf("[INFO] Received %s from %s \n", msg.TypeName(), msg.SenderName)

		if !tracker.AddMessage(msg.ID) {
			continue
		}

		msg.ClusterID = config.Index

		if msg.ReceiverID == config.ClusterConfig.Index && (msg.Type == shared.AkeAMsg || msg.Type == shared.AkeBMsg) {
			clusterTransport.Receive(msg)
		}
		if msg.ReceiverID != config.ClusterConfig.Index && (msg.Type == shared.AkeAMsg || msg.Type == shared.AkeBMsg) {
			sendToClient(msg, clients)
		}
		if msg.Type == shared.XiMsg {
			clusterTransport.Receive(msg)
			broadcastToCluster(msg, clients)
		}
		if msg.Type == shared.TextMsg {
			broadcastToCluster(msg, clients)
			broadcastToLeaders(msg)
		}
		if msg.Type == shared.LeaderAkeAMsg || msg.Type == shared.LeaderAkeBMsg || msg.Type == shared.LeaderXiMsg {
			leaderTransport.Receive(msg)
		}
	}
}
