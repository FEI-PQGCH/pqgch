package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"pqgch/cluster_protocol"
	"pqgch/util"
)

var config util.UserConfig

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()
	if *configFlag == "" {
		log.Fatalln("[ERROR] Configuration file missing. Please provide it using the -config flag")
	}
	config = util.GetUserConfig(*configFlag)

	tui := util.NewTUI()
	tui.HijackStdout()

	transport, err := util.NewTCPTransport(config.LeadAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Unable to connect to server: %v\n", err)
		os.Exit(1)
	}
	transport.Send(util.Message{
		ID:         util.UniqueID(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Type:       util.LoginMsg,
		ClusterID:  config.ClusterConfig.Index,
	})

	session := cluster_protocol.NewSession(transport, &config.ClusterConfig)
	session.Init()

	tui.AttachMessages(session.Received)

	tui.Run(func(line string) {
		session.SendText(line)
	})
}
