package main

import (
	"encoding/base64"
	"fmt"
	"net"
	"pqgch-client/gake"
	"pqgch-client/shared"
)

type MessageHandler interface {
	HandleMessage(conn net.Conn, msg shared.Message)
}

type AkeAHandler struct{}
type AkeBHandler struct{}
type XiHandler struct{}
type DefaultHandler struct{}

func (h *AkeAHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	fmt.Println("CRYPTO: received AKE A message")
	responseMsg := shared.GetAkeSharedBMsg(&session, msg, config.ClusterConfig)
	fmt.Println("CRYPTO: sending AKE B message")
	shared.SendMsg(conn, responseMsg)
	ok, msg := shared.CheckLeftRightKeys(&session, config.ClusterConfig)

	if ok {
		fmt.Println("CRYPTO: sending Xi")
		shared.SendMsg(conn, msg)
	}
}

func (h *AkeBHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	fmt.Println("CRYPTO: received AKE B message")
	akeSendB, _ := base64.StdEncoding.DecodeString(msg.Content)
	session.KeyRight = gake.KexAkeSharedA(akeSendB, session.TkRight, session.EskaRight, config.GetDecodedSecretKey())
	fmt.Println("CRYPTO: established shared key with right neighbor")
	ok, msg := shared.CheckLeftRightKeys(&session, config.ClusterConfig)

	if ok {
		fmt.Println("CRYPTO: sending Xi")
		shared.SendMsg(conn, msg)
	}
}

func (h *XiHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	if msg.SenderID == config.Index {
		return
	}
	fmt.Println("CRYPTO: received Xi")
	xi, _ := base64.StdEncoding.DecodeString(msg.Content)
	var xiArr [32]byte
	copy(xiArr[:], xi)
	session.Xs[msg.SenderID] = xiArr
	shared.CheckXs(&session, config.ClusterConfig)
}

func printMessage(msg shared.Message) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("\r\033[K%s: %s\n", msg.SenderName, msg.Content)
	fmt.Print("You: ")
}

func (h *DefaultHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	var plainText, err = shared.DecryptAesGcm(msg.Content, &session)
	if err != nil {
		fmt.Println("error decrypting message")
		return
	}

	msg.Content = plainText
	printMessage(msg)
}

func GetHandler(msgType int) MessageHandler {
	switch msgType {
	case shared.AkeAMsg:
		return &AkeAHandler{}
	case shared.AkeBMsg:
		return &AkeBHandler{}
	case shared.XiMsg:
		return &XiHandler{}
	default:
		return &DefaultHandler{}
	}
}
