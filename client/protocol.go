package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"pqgch-client/gake"
	"pqgch-client/shared"

	"github.com/google/uuid"
)

// TODO: refactor protocol.go to not use global variables
// TODO: use AES-GCM for encrypting messages after successful protocol execution

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
	fmt.Printf("masterkey%d: %02x\n", config.Index, masterKey)
	skSid := gake.ComputeSkSid(len(config.Names), masterKey, dummyPids)
	fmt.Printf("sksid%d: %02x\n", config.Index, skSid)

	copy(sharedSecret[:], skSid[:32])
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

func EncryptAesGcm(plaintext string, key [32]byte) (string, error) {
	block, err := aes.NewCipher(sharedSecret[:])
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	cipherText := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func DecryptAesGcm(encryptedText string, key []byte) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(cipherText) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, cipherText := cipherText[:nonceSize], cipherText[nonceSize:]

	plainText, err := aesGCM.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}

	return string(plainText), nil
}
