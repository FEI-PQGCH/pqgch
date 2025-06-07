package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"pqgch/cluster_protocol"
	"pqgch/shared"
)

var config shared.UserConfig

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()
	if *configFlag == "" {
		log.Fatalln("[ERROR] Configuration file missing. Please provide it using the -config flag")
	}
	config = shared.GetUserConfig(*configFlag)

	tui := shared.NewTUI()
	tui.HijackStdout()

	transport, err := shared.NewTCPTransport(config.LeadAddr)
	if err != nil {
		fmt.Fprintf(os.Stdout, "[ERROR] Unable to connect to server: %v\n", err)
		os.Exit(1)
	}
	transport.Send(shared.Message{
		ID:         shared.UniqueID(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Type:       shared.LoginMsg,
		ClusterID:  config.ClusterConfig.Index,
	})

	session := cluster_protocol.NewSession(transport, &config.ClusterConfig)
	session.Init()

	tui.AttachMessages(session.Received)

	tui.Run(func(line string) {
		session.SendText(line)
	})
}
