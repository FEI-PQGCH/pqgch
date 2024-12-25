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
type DefaultHandler struct{}

func (h *AkeAHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	sendFunc := func(message shared.Message) {
		shared.SendMsg(conn, message)
	}
	shared.HandleAkeA(msg, &config, &session, sendFunc, sendFunc)
}

func (h *AkeBHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	sendFunc := func(message shared.Message) {
		shared.SendMsg(conn, message)
	}
	shared.HandleAkeB(msg, &config, &session, sendFunc)
}

func (h *XiHandler) HandleMessage(conn net.Conn, msg shared.Message) {
	shared.HandleXi(msg, &config, &session)
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
