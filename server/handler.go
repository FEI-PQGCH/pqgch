package main

import (
	"fmt"
	"net"
	"pqgch-client/shared"
)

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

func handleMessage(conn net.Conn, msg shared.Message) {
	switch msg.Type {
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
