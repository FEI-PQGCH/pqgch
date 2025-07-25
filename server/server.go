package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"pqgch/cluster_protocol"
	"pqgch/leader_protocol"
	"pqgch/util"
)

var config util.LeaderConfig

func main() {
	// Parse command line flag.
	path := flag.String("config", "", "path to configuration file")
	flag.Parse()
	if *path == "" {
		fmt.Fprintf(os.Stderr, "[ERROR] Configuration file missing. Please provide it using the -config flag.\n")
		os.Exit(1)
	}

	// Load config.
	var err error
	config, err = util.GetConfig[util.LeaderConfig](*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Error loading config from : %v\n", err)
		os.Exit(1)
	}

	// Parse port from config.
	_, port, err := net.SplitHostPort(config.Addrs[config.Index])
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Error parsing self port from config: %v\n", err)
		os.Exit(1)
	}

	// Start TCP listener.
	address := fmt.Sprintf(":%s", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Error starting TCP server: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()
	util.PrintLine(fmt.Sprintf("[ROUTE]: server listening on %s\n", address))

	// Create message tracker and in-memory client registry.
	tracker := util.NewMessageTracker()
	clients := newClients(config.ClusterConfig)

	// Initialize leader transport and session.
	var clusterSession *cluster_protocol.Session
	msgsLeader := make(chan util.Message)

	leaderSession := leader_protocol.NewSession(
		newLeaderMessageSender(),
		config,
		func() { clusterSession.OnClusterKey() },
		msgsLeader,
	)

	// Initialize cluster transport and session.
	msgsCluster := make(chan util.Message)

	clusterSession = cluster_protocol.NewLeaderSession(
		newClusterMessageSender(clients),
		config.ClusterConfig,
		leaderSession.GetKeyRef(),
		msgsCluster,
	)

	leaderSession.Init()
	clusterSession.Init()

	go leaderSession.MessageHandler()
	go clusterSession.MessageHandler()

	if config.IsClusterQKDUrl() {
		go func() {
			keyMsg, IDMsg := util.RequestKey(config.ClusterQKDUrl(), false)
			msgsCluster <- keyMsg
			clients.broadcast(IDMsg)
		}()
	}

	if config.IsRightQKDUrl() {
		go func() {
			keyMsg, IDMsg := util.RequestKey(config.RightQKDUrl(), true)
			msgsLeader <- keyMsg
			sendToLeader(config.Addrs[config.RightIndex()], IDMsg)
		}()
	}

	// Accept connections (from cluster members or other leaders) in a goroutine.
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				util.PrintLine(fmt.Sprintf("[ERROR] Error accepting connection: %v\n", err))
				continue
			}
			go handleConnection(clients, conn, tracker, msgsCluster, msgsLeader)
		}
	}()

	// Run the TUI event loop; on ENTER send text via cluster session.
	util.StartTUI(func(line string) {
		clusterSession.SendText(line)
	})
}

func handleConnection(
	clients *Clients,
	conn net.Conn,
	tracker *util.MessageTracker,
	clusterChan chan util.Message,
	leaderChan chan util.Message,
) {
	defer conn.Close()

	reader := util.NewMessageReader(conn)
	// Verify that the client sent some message.
	if !reader.HasMessage() {
		util.PrintLine("[ERROR] Client did not send any message")
		return
	}

	// Check whether message is a Login message.
	// If it is not, we received a message from some other cluster leader.
	// We handle the Text message explicitly by broadcasting it to our cluster.
	// We handle other messages through Leader Transport since they should
	// be a part of the protocol between leaders.
	msg := reader.GetMessage()
	if msg.Type != util.LoginMsg {
		if !tracker.AddMessage(msg.ID) {
			return
		}
		// Log receipt of a leader protocol message.
		util.PrintLine(fmt.Sprintf("[ROUTE] Received %s message from Leader\n", msg.TypeName()))
		if msg.Type == util.TextMsg {
			clusterChan <- msg
			clients.broadcast(msg)
			return
		}
		if msg.Type == util.QKDIDsMsg {
			go func() {
				msg := util.RequestKeyByID(config.LeftQKDUrl(), msg.Content)
				leaderChan <- msg
			}()
			return
		}
		leaderChan <- msg
		return
	}

	// Handle client login.
	util.PrintLine(fmt.Sprintf("[INFO] New client (%s, %s) joined\n", msg.SenderName, conn.RemoteAddr()))
	clientID := msg.SenderID

	clients.makeOnline(clientID, conn)
	defer clients.makeOffline(clientID)

	// Send to the newly connected client every message from its queue.
	clients.sendQueued(clientID)

	// Handle messages from this client in an infinite loop.
	for reader.HasMessage() {
		msg := reader.GetMessage()
		util.PrintLine(fmt.Sprintf("[ROUTE] Received %s from %s\n", msg.TypeName(), msg.SenderName))

		if !tracker.AddMessage(msg.ID) {
			continue
		}

		msg.ClusterID = config.Index
		switch msg.Type {
		case util.AkeOneMsg, util.AkeTwoMsg:
			if msg.ReceiverID == config.ClusterConfig.Index {
				clusterChan <- msg
			} else {
				clients.send(msg)
			}
		case util.XiRiCommitmentMsg:
			clusterChan <- msg
			clients.broadcast(msg)
		case util.TextMsg:
			clusterChan <- msg
			clients.broadcast(msg)
			broadcastToLeaders(msg)
		case util.LeadAkeOneMsg, util.LeadAkeTwoMsg, util.LeaderXiRiCommitmentMsg:
			leaderChan <- msg
		}
	}
}
