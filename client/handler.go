package main

import (
	"encoding/base64"
	"fmt"
	"net"
	"pqgch-client/gake"
	"pqgch-client/shared"
)

// TODO: refactor protocol.go to not use global variables (except for config)

type MessageHandler interface {
	HandleMessage(conn net.Conn, msg shared.Message)
}

type AkeSendAHandler struct{}
type AkeSendBHandler struct{}
type IntraBroadcastHandler struct{}
type DefaultHandler struct{}

func (h *AkeSendAHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	fmt.Println("received AKE A message")
	responseMsg := shared.GetAkeSharedBMsg(msg, config.ClusterConfig, &keyLeft)
	fmt.Println("sending AKE B message")
	shared.SendMsg(conn, responseMsg)
	ok, msg := shared.CheckLeftRightKeys(&keyRight, &keyLeft, &Xs, config.ClusterConfig, &sharedSecret)

	if ok {
		fmt.Println("sending Xi")
		shared.SendMsg(conn, msg)
	}
}

func (h *AkeSendBHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	fmt.Println("received AKE B message")
	akeSendB, _ := base64.StdEncoding.DecodeString(msg.Content)
	keyRight = gake.KexAkeSharedA(akeSendB, tkRight, eskaRight, config.GetDecodedSecretKey())
	fmt.Println("established shared key with right neighbor")
	ok, msg := shared.CheckLeftRightKeys(&keyRight, &keyLeft, &Xs, config.ClusterConfig, &sharedSecret)

	if ok {
		fmt.Println("sending Xi")
		shared.SendMsg(conn, msg)
	}
}

func (h *IntraBroadcastHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	if msg.SenderID == config.Index {
		return
	}
	fmt.Println("received Xi")
	xi, _ := base64.StdEncoding.DecodeString(msg.Content)
	var xiArr [32]byte
	copy(xiArr[:], xi)
	Xs[msg.SenderID] = xiArr
	shared.CheckXs(&Xs, config.ClusterConfig, &keyLeft, &sharedSecret)
}

func printMessage(msg shared.Message) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("\r\033[K%s: %s\n", msg.SenderName, msg.Content)
	fmt.Print("You: ")
}

func (h *DefaultHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	var plainText, err = shared.DecryptAesGcm(msg.Content, sharedSecret[:])
	if err != nil {
		fmt.Println("error decrypting message")
		return
	}

	msg.Content = plainText
	printMessage(msg)
}

func GetHandler(msgType int) MessageHandler {
	switch msgType {
	case shared.MsgAkeSendA:
		return &AkeSendAHandler{}
	case shared.MsgAkeSendB:
		return &AkeSendBHandler{}
	case shared.MsgIntraBroadcast:
		return &IntraBroadcastHandler{}
	default:
		return &DefaultHandler{}
	}
}
