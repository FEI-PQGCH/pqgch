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

	// Load config.
	if *configFlag != "" {
		config = shared.GetServConfig(*configFlag)
	} else {
		fmt.Printf("[ERROR] Configuration file missing. Please provide it using the -config flag.\n")
		os.Exit(1)
	}

	// Start listening at configured port.
	_, port, err := net.SplitHostPort(config.GetCurrentServer())
	if err != nil {
		fmt.Println("[ERROR] Error parsing self address from config:", err)
		os.Exit(1)
	}
	address := fmt.Sprintf(":%s", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("[ERROR] Error starting TCP server:", err)
		os.Exit(1)
	}
	defer listener.Close()
	fmt.Println("ROUTE: server listening on", address)

	// Create message tracker so we do not process the same message twice.
	tracker := shared.NewMessageTracker()

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

	// Initialize transports and sessions.
	leaderTransport := NewLeaderTransport()
	leaderSession := leader_protocol.NewSession(leaderTransport, &config)
	clusterTransport := NewClusterTransport(&clients)
	clusterSession := cluster_protocol.NewLeaderSession(clusterTransport, &config.ClusterConfig, leaderSession.GetKeyRef())

	// Start the protocol between cluster leaders.
	leaderSession.Init()

	// Handle incoming connections.
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
	session *cluster_protocol.Session,
	clusterTransport *ClusterTransport,
	leaderTransport *LeaderTransport) {
	reader := shared.NewMessageReader(conn)
	// Verify that the client sent some message.
	if !reader.HasMessage() {
		fmt.Println("[ERROR] Client did not send any message")
		conn.Close()
		return
	}

	// Check whether message is a Login message.
	// If it is not, we received a message from some other cluster leader.
	// We handle the Text message explicitly by broadcasting it to our cluster.
	// We handle other messages through Leader Transport since they shoould
	// be a part of the protocol between leaders.
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

	// If the message sent was a Login message (which comes from clients), we save the client and his connection in memory.
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

	// Send to the newly connected client every message from his queue.
	for _, msg := range clients.cs[clientID].queue {
		msg.Send(clients.cs[clientID].conn)
		clients.mu.Lock()
		clients.cs[clientID].queue.Remove(msg)
		clients.mu.Unlock()
	}

	// Check whether all the clients are connected.
	// If so, begin the cluster protocol from the Leader side.
	//
	// TODO: refactor this. We do not need to wait until everyone joined, we only need to start the protocol
	// when our right neighbour (in the cluster) joined. Then, if we try to deliver some messages to offline clients,
	// we just store them in their queues.
	onlineClients := 0
	for _, c := range clients.cs {
		if c.conn != nil {
			onlineClients++
		}
	}
	if onlineClients == len(config.Names)-1 {
		session.Init()
	}

	// Handle messages from this client in an infinite loop.
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
		if msg.Type == shared.XiRiCommitmentMsg {
			clusterTransport.Receive(msg)
			broadcastToCluster(msg, clients)
		}
		if msg.Type == shared.TextMsg {
			broadcastToCluster(msg, clients)
			broadcastToLeaders(msg)
		}
		if msg.Type == shared.LeaderAkeAMsg || msg.Type == shared.LeaderAkeBMsg || msg.Type == shared.LeaderXiRiCommitmentMsg {
			leaderTransport.Receive(msg)
		}
	}
}
