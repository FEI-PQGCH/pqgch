package main

import (
	"flag"
	"fmt"
	"os"
	"pqgch/cluster_protocol"
	"pqgch/util"
)

func main() {
	// Parse command line flag.
	path := flag.String("config", "", "path to configuration file")
	flag.Parse()
	if *path == "" {
		fmt.Println("Configuration file missing. Please provide it using the -config flag")
		os.Exit(1)
	}

	// Load config.
	config, err := util.GetConfig[util.MemberConfig](*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config from : %v\n", err)
		os.Exit(1)
	}

	// Initialize TCP transport.
	msgChan := make(chan util.Message)
	transport, err := util.NewTCPTransport(config.Server, msgChan)
	if err != nil {
		fmt.Printf("Unable to connect to server: %v\n", err)
		os.Exit(1)
	}

	util.EnableRawMode()
	transport.Send(util.Message{
		ID:         util.UniqueID(),
		SenderID:   config.ClusterConfig.MemberID,
		SenderName: config.ClusterConfig.Name(),
		Type:       util.LoginMsg,
		ClusterID:  config.ClusterConfig.ClusterID,
	})

	// Initialize cluster protocol session.
	session := cluster_protocol.NewSession(transport, config.ClusterConfig, msgChan)
	session.Init()
	go session.MessageHandler()

	// Start Terminal User Interface.
	util.StartTUI(func(line string) {
		session.SendText(line)
	})
}
