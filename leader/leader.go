package main

import (
	"flag"
	"fmt"
	"os"
	"pqgch/cluster_protocol"
	"pqgch/leader_protocol"
	"pqgch/util"
)

// Demultiplex received messages into their corresponding sessions.
func demuxMessages(in <-chan util.Message) (chan util.Message, chan util.Message) {
	cluster := make(chan util.Message)
	leader := make(chan util.Message)

	go func() {
		defer close(cluster)
		defer close(leader)
		for msg := range in {
			if msg.IsClusterType() {
				cluster <- msg
			} else {
				leader <- msg
			}
		}
	}()

	return cluster, leader
}

func main() {
	// Parse command line flag for configuration filename.
	path := flag.String("config", "", "path to configuration file")
	flag.Parse()
	if *path == "" {
		fmt.Fprintf(os.Stderr, "configuration file missing, please provide it using the -config flag\n")
		os.Exit(1)
	}

	// Load config.
	config, err := util.GetConfig(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize TCP transport.
	msgChan := make(chan util.Message)
	transport, err := util.NewTCPTransport(config.Server, msgChan, config.GetMemberID(), *config.ClusterID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to connect to routing server (is it running?): %v\n", err)
		os.Exit(1)
	}

	// Log in to the routing server.
	transport.Send(util.Message{
		SenderID:   config.GetMemberID(),
		SenderName: config.Name,
		Type:       util.LeaderAuthMsg,
		ClusterID:  *config.ClusterID,
	})

	// Create channels for both sessions.
	msgsCluster, msgsLeader := demuxMessages(msgChan)

	// Initialize cluster transport and session.
	clusterSession := cluster_protocol.NewLeaderSession(
		transport,
		config,
		msgsCluster,
	)

	// Initialize leader transport and session.
	leaderSession := leader_protocol.NewSession(
		transport,
		config,
		msgsCluster,
		msgsLeader,
	)

	leaderSession.Init()
	clusterSession.Init()

	// Start handling messages.
	go leaderSession.MessageHandler()
	go clusterSession.MessageHandler()

	if config.Cluster.HasQKDUrl() {
		go func() {
			key, keyID := util.RequestKey(config.Cluster.QKDUrl())

			msgsCluster <- util.Message{
				Type:    util.QKDClusterKeyMsg,
				Content: key,
			}

			transport.Send(util.Message{
				ClusterID:  *config.ClusterID,
				SenderID:   config.GetMemberID(),
				SenderName: config.Name,
				Type:       util.QKDIDMemberMsg,
				Content:    keyID,
			})
		}()
	}

	// If two leaders use QKD, one of them (this one) fetches the key
	// and sends the key ID to his right neighbor.
	if config.Leader.HasRightQKDUrl() {
		go func() {
			key, keyID := util.RequestKey(config.Leader.RightQKDUrl())

			msgsLeader <- util.Message{
				Type:    util.QKDRightKeyMsg,
				Content: key,
			}

			transport.Send(util.Message{
				ClusterID:  *config.ClusterID,
				SenderID:   config.GetMemberID(),
				ReceiverID: config.RightClusterID(),
				SenderName: config.Name,
				Type:       util.QKDIDLeaderMsg,
				Content:    keyID,
			})
		}()
	}

	// Start Terminal User Interface.
	util.StartTUI(func(line string) {
		clusterSession.SendText(line)
	})
}
