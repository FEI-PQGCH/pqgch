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
		fmt.Fprintf(os.Stderr, "Configuration file missing. Please provide it using the -config flag.\n")
		os.Exit(1)
	}

	// Load config.
	var err error
	config, err = util.GetConfig[util.LeaderConfig](*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config from : %v\n", err)
		os.Exit(1)
	}

	// Parse port from config.
	_, port, err := net.SplitHostPort(config.Addrs[config.Index])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing self port from config: %v\n", err)
		os.Exit(1)
	}

	// Start TCP listener.
	address := fmt.Sprintf(":%s", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting TCP server: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()
	util.EnableRawMode()
	util.LogInfo(fmt.Sprintf("Server listening on %s", address))

	// Create message tracker and in-memory client registry.
	tracker := util.NewMessageTracker()
	clients := newClients(config.ClusterConfig)

	// Initialize cluster transport and session.
	msgsCluster := make(chan util.Message)
	clusterSession := cluster_protocol.NewLeaderSession(
		newClusterMessageSender(clients),
		config.ClusterConfig,
		msgsCluster,
	)

	// Initialize leader transport and session.
	msgsLeader := make(chan util.Message)
	leaderSession := leader_protocol.NewSession(
		newLeaderMessageSender(),
		config,
		msgsCluster,
		msgsLeader,
	)

	leaderSession.Init()
	clusterSession.Init()

	// Start handling messages.
	go leaderSession.MessageHandler()
	go clusterSession.MessageHandler()

	// If the cluster uses QKD, the leader fetches the key
	// and sends the key ID to the cluster members.
	if config.ClusterConfig.IsClusterQKDUrl() {
		go func() {
			keyMsg, IDMsg := util.RequestKey(config.ClusterConfig.ClusterQKDUrl(), false)
			msgsCluster <- keyMsg
			IDMsg.SenderName = config.ClusterConfig.Name()
			clients.broadcast(IDMsg)
		}()
	}

	// If two leaders use QKD, one of them (this one) fetches the key
	// and sends the key ID to his right neighbor.
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
				util.LogError(fmt.Sprintf("Error accepting connection: %v", err))
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
		util.LogError("Client did not send any message")
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
		util.LogRoute(fmt.Sprintf("Received %s from Leader %s", msg.TypeName(), config.Addrs[msg.ClusterID]))
		if msg.Type == util.TextMsg {
			clusterChan <- msg
			clients.broadcast(msg)
			return
		}
		if msg.Type == util.QKDIDMsg {
			go func() {
				msg := util.RequestKeyByID(config.LeftQKDUrl(), msg.Content, true)
				leaderChan <- msg
			}()
			return
		}
		leaderChan <- msg
		return
	}

	// Handle client login.
	util.LogInfo(fmt.Sprintf("New client (%s, %s) joined", msg.SenderName, conn.RemoteAddr()))
	clientID := msg.SenderID

	clients.makeOnline(clientID, conn)
	defer clients.makeOffline(clientID)

	// Send to the newly connected client every message from its queue.
	clients.sendQueued(clientID)

	// Handle messages from this client in an infinite loop.
	for reader.HasMessage() {
		msg := reader.GetMessage()
		util.LogRoute(fmt.Sprintf("Received %s from %s", msg.TypeName(), msg.SenderName))

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
