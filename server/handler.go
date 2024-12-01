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

type AkeAHandler struct{}
type AkeBHandler struct{}
type XiHandler struct{}
type BroadcastHandler struct{}
type SpecificClientHandler struct{}
type DefaultHandler struct{}

func (h *AkeAHandler) HandleMessage(msg shared.Message) {
	fmt.Println("CRYPTO: received AKE A message")
	responseMsg := shared.GetAkeBMsg(&clusterSession, msg, &config.ClusterConfig)
	fmt.Println("CRYPTO: sending AKE B message")
	sendMsgToClient(responseMsg)
	ok, msg := shared.CheckLeftRightKeys(&clusterSession, &config.ClusterConfig)

	if ok {
		fmt.Println("CRYPTO: sending Xi")
		broadcastMessage(msg)
	}
}

func (h *AkeBHandler) HandleMessage(msg shared.Message) {
	fmt.Println("CRYPTO: received AKE B message")
	akeSendB, _ := base64.StdEncoding.DecodeString(msg.Content)
	clusterSession.KeyRight = gake.KexAkeSharedA(akeSendB, clusterSession.TkRight, clusterSession.EskaRight, config.ClusterConfig.GetDecodedSecretKey())
	fmt.Println("CRYPTO: established shared key with right neighbor")
	ok, msg := shared.CheckLeftRightKeys(&clusterSession, &config.ClusterConfig)

	if ok {
		fmt.Println("CRYPTO: sending Xi")
		broadcastMessage(msg)
	}
}

func (h *XiHandler) HandleMessage(msg shared.Message) {
	if msg.SenderID == config.ClusterConfig.Index {
		return
	}
	fmt.Println("CRYPTO: received Xi")
	xi, _ := base64.StdEncoding.DecodeString(msg.Content)
	var xiArr [32]byte
	copy(xiArr[:], xi)
	clusterSession.Xs[msg.SenderID] = xiArr
	shared.CheckXs(&clusterSession, &config.ClusterConfig)
	broadcastMessage(msg)
}

func (h *BroadcastHandler) HandleMessage(msg shared.Message) {
	broadcastMessage(msg)
	forwardMessage(msg)
}

func (h *DefaultHandler) HandleMessage(msg shared.Message) {
	var plainText, err = shared.DecryptAesGcm(msg.Content, &clusterSession)
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
		if msg.Type == shared.AkeAMsg {
			return &AkeAHandler{}
		}
		if msg.Type == shared.AkeBMsg {
			return &AkeBHandler{}
		}
	}

	if msg.Type == shared.XiMsg {
		return &XiHandler{}
	}

	if msg.Type == shared.BroadcastMsg {
		return &BroadcastHandler{}
	}

	if msg.Type == shared.AkeAMsg || msg.Type == shared.AkeBMsg {
		return &SpecificClientHandler{}
	}

	return &DefaultHandler{}
}
