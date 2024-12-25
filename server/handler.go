package main

import (
	"fmt"
	"net"
	"pqgch-client/shared"
)

type MessageHandler interface {
	HandleMessage(conn net.Conn, msg shared.Message)
}

type AkeAHandler struct{}
type AkeBHandler struct{}
type XiHandler struct{}
type LeaderAkeAHandler struct{}
type LeaderAkeBHandler struct{}
type LeaderXiHandler struct{}
type BroadcastHandler struct{}
type SpecificClientHandler struct{}
type DefaultHandler struct{}

func (h *AkeAHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	shared.HandleAkeA(msg, &config.ClusterConfig, &clusterSession, sendMsgToClient, broadcastMessage)
}

func (h *AkeBHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	shared.HandleAkeB(msg, &config.ClusterConfig, &clusterSession, broadcastMessage)
}

func (h *XiHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	shared.HandleXi(msg, &config.ClusterConfig, &clusterSession)
	broadcastMessage(msg)
}

func (h *LeaderAkeAHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	sendFunc := func(message shared.Message) {
		message.Send(conn)
	}
	shared.HandleAkeA(msg, &config, &mainSession, sendFunc, forwardMessage)
}

func (h *LeaderAkeBHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	shared.HandleAkeB(msg, &config, &mainSession, forwardMessage)
}

func (h *LeaderXiHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	shared.HandleXi(msg, &config, &mainSession)
	forwardMessage(msg)
}

func (h *BroadcastHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	broadcastMessage(msg)
	forwardMessage(msg)
}

func (h *DefaultHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	fmt.Println("error: unknown message type")
}

func (h *SpecificClientHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	sendMsgToClient(msg)
}

func GetHandler(msg shared.Message) MessageHandler {
	if msg.ReceiverID == config.ClusterConfig.Index {
		if msg.Type == shared.AkeAMsg {
			return &AkeAHandler{}
		}
		if msg.Type == shared.AkeBMsg {
			return &AkeBHandler{}
		}
	}

	switch msg.Type {
	case shared.XiMsg:
		return &XiHandler{}
	case shared.BroadcastMsg:
		return &BroadcastHandler{}
	case shared.AkeAMsg, shared.AkeBMsg:
		return &SpecificClientHandler{}
	case shared.LeaderAkeAMsg:
		return &LeaderAkeAHandler{}
	case shared.LeaderAkeBMsg:
		return &LeaderAkeBHandler{}
	case shared.LeaderXiMsg:
		return &LeaderXiHandler{}
	}

	return &DefaultHandler{}
}
