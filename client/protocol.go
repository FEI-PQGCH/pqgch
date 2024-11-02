package main

import (
	"encoding/base64"
	"fmt"
	"pqgch-client/gake"
	"pqgch-client/shared"

	"github.com/google/uuid"
)

// TODO: refactor protocol.go to not use global variables

func CheckLeftRightKeys() (bool, shared.Message) {
	if keyRight != [32]byte{} && keyLeft != [32]byte{} {
		fmt.Println("established shared keys with both neighbors")
		msg := GetXiMsg()
		CheckXs()
		return true, msg
	}
	return false, shared.Message{}
}

func GetXiMsg() shared.Message {
	xi, _, _ := gake.ComputeXsCommitment(
		config.Index,
		keyRight,
		keyLeft,
		config.GetDecodedPublicKey((config.Index+1)%len(config.Names)))

	var xiArr [32]byte
	copy(xiArr[:], xi)
	Xs[config.Index] = xiArr
	msg := shared.Message{
		MsgID:      uuid.New().String(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		MsgType:    shared.MsgIntraBroadcast,
		Content:    base64.StdEncoding.EncodeToString(xi[:]),
	}

	return msg
}

func CheckXs() {
	for i := 0; i < len(Xs); i++ {
		if Xs[i] == [32]byte{} {
			return
		}
	}

	fmt.Println("received all Xs")
	for i := 0; i < len(Xs); i++ {
		fmt.Printf("X%d: %02x\n", i, Xs[i])
	}
	dummyPids := make([][20]byte, len(config.Names)) // TODO: replace with actual pids

	masterKey := gake.ComputeMasterKey(len(config.Names), config.Index, keyLeft, Xs)
	fmt.Printf("masterkey%d: %02x\n\n", config.Index, masterKey)
	sksid := gake.ComputeSkSid(len(config.Names), masterKey, dummyPids)
	fmt.Printf("sksid%d: %02x\n\n", config.Index, sksid)
}

func GetAkeInitAMsg() shared.Message {
	var rightIndex = (config.Index + 1) % len(config.Names)
	var akeSendARight []byte
	akeSendARight, tkRight, eskaRight = gake.KexAkeInitA(config.GetDecodedPublicKey(rightIndex))

	msg := shared.Message{
		MsgID:      uuid.New().String(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		MsgType:    shared.MsgAkeSendA,
		ReceiverID: rightIndex,
		Content:    base64.StdEncoding.EncodeToString(akeSendARight),
	}

	return msg
}

func GetAkeSharedBMsg(msg shared.Message) shared.Message {
	akeSendA, _ := base64.StdEncoding.DecodeString(msg.Content)

	var akeSendB []byte
	akeSendB, keyLeft = gake.KexAkeSharedB(
		akeSendA,
		config.GetDecodedSecretKey(),
		config.GetDecodedPublicKey(msg.SenderID))

	fmt.Println("established shared key with left neighbor")

	msg = shared.Message{
		MsgID:      uuid.New().String(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		MsgType:    shared.MsgAkeSendB,
		ReceiverID: msg.SenderID,
		Content:    base64.StdEncoding.EncodeToString(akeSendB),
	}

	return msg
}
