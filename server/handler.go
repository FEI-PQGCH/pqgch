package main

import (
	"fmt"
	"net"
	"pqgch-client/shared"
)

func onClusterSession() {
	fmt.Println("[CRYPTO] Broadcasting master key to cluster")
	var wrappingKey [64]byte
	if useExternal {
		copy(wrappingKey[:], intraClusterKey[:])
	} else {
		copy(wrappingKey[:], clusterSession.SharedSecret[:])
	}
	keyMsg := shared.EncryptAndHMAC(mainSession.SharedSecret[:], &config, wrappingKey[:])
	broadcastToCluster(keyMsg)
}

func akeA(msg shared.Message) {
	akeB, xi := shared.HandleAkeA(msg, &config.ClusterConfig, &clusterSession)
	sendToClient(akeB)
	if !xi.IsEmpty() {
		broadcastToCluster(xi)
	}
}

func akeB(msg shared.Message) {
	xi := shared.HandleAkeB(msg, &config.ClusterConfig, &clusterSession)
	if !xi.IsEmpty() {
		broadcastToCluster(xi)
	}
}

func xi(msg shared.Message) {
	shared.HandleXi(msg, &config.ClusterConfig, &clusterSession)
	broadcastToCluster(msg)
}

func akeALeader(conn net.Conn, msg shared.Message) {
	akeB, xi := shared.HandleAkeA(msg, &config, &mainSession)
	akeB.Send(conn)
	if !xi.IsEmpty() {
		forwardToNeighbor(xi)
	}
}

func akeBLeader(msg shared.Message) {
	xi := shared.HandleAkeB(msg, &config, &mainSession)
	if !xi.IsEmpty() {
		forwardToNeighbor(xi)
	}
}

func xiLeader(msg shared.Message) {
	shared.HandleXi(msg, &config, &mainSession)
	forwardToNeighbor(msg)
}

func broadcast(msg shared.Message) {
	broadcastToCluster(msg)
	forwardToNeighbor(msg)
}

func handleMessage(conn net.Conn, msg shared.Message) {
	if msg.ReceiverID == config.ClusterConfig.Index {
		switch msg.Type {
		case shared.AkeAMsg:
			akeA(msg)
			return
		case shared.AkeBMsg:
			akeB(msg)
			return
		}
	}

	switch msg.Type {
	case shared.XiMsg:
		xi(msg)
	case shared.BroadcastMsg:
		broadcast(msg)
	case shared.AkeAMsg, shared.AkeBMsg:
		sendToClient(msg)
	case shared.LeaderAkeAMsg:
		akeALeader(conn, msg)
	case shared.LeaderAkeBMsg:
		akeBLeader(msg)
	case shared.LeaderXiMsg:
		xiLeader(msg)
	default:
		fmt.Printf("[ERROR] Unknown message type encountered\n")
	}
}
