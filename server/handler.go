package main

import (
	"encoding/base64"
	"fmt"
	"pqgch-client/gake"
	"pqgch-client/shared"
)

type MessageHandler interface {
	HandleMessage(msg shared.Message)
}

type AkeSendAHandler struct{}
type AkeSendBHandler struct{}
type IntraBroadcastHandler struct{}
type BroadcastHandler struct{}
type SpecificClientHandler struct{}
type DefaultHandler struct{}

func (h *AkeSendAHandler) HandleMessage(msg shared.Message) {
	fmt.Println("received AKE A message")
	responseMsg := shared.GetAkeSharedBMsg(msg, config.ClusterConfig, &keyLeft)
	fmt.Println("sending AKE B message")
	sendMsgToClient(responseMsg)
	ok, msg := shared.CheckLeftRightKeys(&keyRight, &keyLeft, &Xs, config.ClusterConfig, &sharedSecret)

	if ok {
		fmt.Println("sending Xi")
		broadcastMessage(msg)
	}
}

func (h *AkeSendBHandler) HandleMessage(msg shared.Message) {
	fmt.Println("received AKE B message")
	akeSendB, _ := base64.StdEncoding.DecodeString(msg.Content)
	keyRight = gake.KexAkeSharedA(akeSendB, tkRight, eskaRight, config.GetDecodedSecretKey())
	fmt.Println("established shared key with right neighbor")
	ok, msg := shared.CheckLeftRightKeys(&keyRight, &keyLeft, &Xs, config.ClusterConfig, &sharedSecret)

	if ok {
		fmt.Println("sending Xi")
		broadcastMessage(msg)
	}
}

func (h *IntraBroadcastHandler) HandleMessage(msg shared.Message) {
	fmt.Println("received intra-broadcast message")
	if msg.SenderID == config.ClusterConfig.Index {
		return
	}
	fmt.Println("received Xi")
	xi, _ := base64.StdEncoding.DecodeString(msg.Content)
	fmt.Printf("xi: %02x\n", xi)
	var xiArr [32]byte
	copy(xiArr[:], xi)
	Xs[msg.SenderID] = xiArr
	shared.CheckXs(&Xs, config.ClusterConfig, &keyLeft, &sharedSecret)
	fmt.Printf("sharedSecret: %02x\n", sharedSecret)
	broadcastMessage(msg)
}

func (h *BroadcastHandler) HandleMessage(msg shared.Message) {
	broadcastMessage(msg)
	forwardMessage(msg)
}

func (h *DefaultHandler) HandleMessage(msg shared.Message) {
	var plainText, err = shared.DecryptAesGcm(msg.Content, sharedSecret[:])
	if err != nil {
		fmt.Println("error decrypting message")
		return
	}

	msg.Content = plainText
	printMessage(msg)
}

func (h *SpecificClientHandler) HandleMessage(msg shared.Message) {
	sendMsgToClient(msg)
}

func printMessage(msg shared.Message) {
	// mu.Lock()
	// defer mu.Unlock()
	// fmt.Printf("\r\033[K%s: %s\n", msg.SenderName, msg.Content)
	// fmt.Print("You: ")
}

func GetHandler(msg shared.Message) MessageHandler {
	if msg.ReceiverID == config.ClusterConfig.Index {
		if msg.MsgType == shared.MsgAkeSendA {
			return &AkeSendAHandler{}
		}
		if msg.MsgType == shared.MsgAkeSendB {
			return &AkeSendBHandler{}
		}
	}

	if msg.MsgType == shared.MsgIntraBroadcast {
		return &IntraBroadcastHandler{}
	}

	if msg.MsgType == shared.MsgBroadcast {
		return &BroadcastHandler{}
	}

	if msg.MsgType == shared.MsgAkeSendA || msg.MsgType == shared.MsgAkeSendB {
		return &SpecificClientHandler{}
	}

	return &DefaultHandler{}
}
