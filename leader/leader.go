package main

import (
	"flag"
	"fmt"
	"os"
	"pqgch/cluster_protocol"
	"pqgch/leader_protocol"
	"pqgch/util"
)

func demux(in <-chan util.Message) (chan util.Message, chan util.Message) {
	cluster := make(chan util.Message)
	leader := make(chan util.Message)

	go func() {
		defer close(cluster)
		defer close(leader)
		for msg := range in {
			switch msg.Type {
			case util.AkeOneMsg:
				fallthrough
			case util.AkeTwoMsg:
				fallthrough
			case util.XiRiCommitmentMsg:
				fallthrough
			case util.KeyMsg:
				fallthrough
			case util.MainSessionKeyMsg:
				fallthrough
			case util.QKDClusterKeyMsg:
				fallthrough
			case util.TextMsg:
				cluster <- msg
			case util.LeadAkeOneMsg:
				fallthrough
			case util.LeadAkeTwoMsg:
				fallthrough
			case util.LeaderXiRiCommitmentMsg:
				fallthrough
			case util.QKDLeftKeyMsg:
				fallthrough
			case util.QKDRightKeyMsg:
				fallthrough
			case util.QKDIDLeaderMsg:
				leader <- msg
			default:
				util.LogError("Unknown message type encountered")
			}
		}
	}()

	return cluster, leader
}

func main() {
	// Parse command line flag.
	path := flag.String("config", "", "path to configuration file")
	flag.Parse()
	if *path == "" {
		fmt.Fprintf(os.Stderr, "configuration file missing, please provide it using the -config flag\n")
		os.Exit(1)
	}

	// Load config.
	config, err := util.GetConfig[util.BaseConfig](*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize TCP transport.
	msgChan := make(chan util.Message)
	transport, err := util.NewTCPTransport(config.Server, msgChan, config.Cluster.MemberID, config.ClusterID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to connect to server: %v\n", err)
		os.Exit(1)
	}

	util.EnableRawMode()
	transport.Send(util.Message{
		SenderID:   config.Cluster.MemberID,
		SenderName: config.Name,
		Type:       util.LeaderAuthMsg,
		ClusterID:  config.ClusterID,
	})

	msgsCluster, msgsLeader := demux(msgChan)

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
				ClusterID:  config.ClusterID,
				SenderID:   config.Cluster.MemberID,
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
				ClusterID:  config.ClusterID,
				SenderID:   config.Cluster.MemberID,
				ReceiverID: config.RightClusterID(),
				SenderName: config.Name,
				Type:       util.QKDIDLeaderMsg,
				Content:    keyID,
			})
		}()
	}

	// Run the TUI event loop; on ENTER send text via cluster session.
	util.StartTUI(func(line string) {
		clusterSession.SendText(line)
	})
}
