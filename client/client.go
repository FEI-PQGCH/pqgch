package main

import (
	"flag"
	"fmt"
	"os"
	"pqgch/cluster_protocol"
	"pqgch/util"
)

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()
	if *configFlag == "" {
		util.PrintLine(fmt.Sprintln(
			"[ERROR] Configuration file missing. Please provide it using the -config flag",
		))
		os.Exit(1)
	}
	config, _ := util.GetConfig[util.UserConfig](*configFlag)

	transport, err := util.NewTCPTransport(config.LeadAddr)
	if err != nil {
		util.PrintLine(fmt.Sprintf("[ERROR] Unable to connect to server: %v\n", err))
		os.Exit(1)
	}
	transport.Send(util.Message{
		ID:         util.UniqueID(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Type:       util.LoginMsg,
		ClusterID:  config.ClusterConfig.Index,
	})

	session := cluster_protocol.NewSession(transport, config.ClusterConfig)
	session.Init()

	util.StartTUI(func(line string) {
		session.SendText(line)
	})
}
