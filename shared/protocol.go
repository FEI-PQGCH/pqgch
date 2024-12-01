package shared

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"pqgch-client/gake"

	"github.com/google/uuid"
)

type Session struct {
	TkRight      []byte
	EskaRight    []byte
	KeyLeft      [32]byte
	KeyRight     [32]byte
	Xs           [][32]byte
	SharedSecret [32]byte
}

func CheckLeftRightKeys(session *Session, config ClusterConfig) (bool, Message) {
	if session.KeyRight != [32]byte{} && session.KeyLeft != [32]byte{} {
		fmt.Println("established shared keys with both neighbors")
		msg := GetXiMsg(session, config)
		CheckXs(session, config)
		return true, msg
	}
	return false, Message{}
}

func GetXiMsg(session *Session, config ClusterConfig) Message {
	xi, _, _ /* TODO: mi1, mi2 */ := gake.ComputeXsCommitment(
		config.Index,
		session.KeyRight,
		session.KeyLeft,
		config.GetDecodedPublicKey((config.Index+1)%len(config.Names)))

	var xiArr [32]byte
	copy(xiArr[:], xi)
	(session.Xs)[config.Index] = xiArr
	msg := Message{
		MsgID:      uuid.New().String(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		MsgType:    MsgIntraBroadcast,
		Content:    base64.StdEncoding.EncodeToString(xi[:]),
	}

	return msg
}

func CheckXs(session *Session, config ClusterConfig) {
	for i := 0; i < len(session.Xs); i++ {
		if (session.Xs)[i] == [32]byte{} {
			return
		}
	}

	fmt.Println("received all Xs")
	for i := 0; i < len(session.Xs); i++ {
		fmt.Printf("X%d: %02x\n", i, (session.Xs)[i])
	}
	dummyPids := make([][20]byte, len(config.Names)) // TODO: replace with actual pids

	masterKey := gake.ComputeMasterKey(len(config.Names), config.Index, session.KeyLeft, session.Xs)
	fmt.Printf("masterkey%d: %02x\n", config.Index, masterKey)
	skSid := gake.ComputeSkSid(len(config.Names), masterKey, dummyPids)
	fmt.Printf("sksid%d: %02x\n", config.Index, skSid)

	copy(session.SharedSecret[:], skSid[:32])
}

func GetAkeInitAMsg(session *Session, config ClusterConfig) Message {
	var rightIndex = (config.Index + 1) % len(config.Names)
	var akeSendARight []byte
	akeSendARight, session.TkRight, session.EskaRight = gake.KexAkeInitA(config.GetDecodedPublicKey(rightIndex))

	msg := Message{
		MsgID:      uuid.New().String(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		MsgType:    MsgAkeSendA,
		ReceiverID: rightIndex,
		Content:    base64.StdEncoding.EncodeToString(akeSendARight),
	}

	return msg
}

func GetAkeSharedBMsg(session *Session, msg Message, config ClusterConfig) Message {
	akeSendA, _ := base64.StdEncoding.DecodeString(msg.Content)

	var akeSendB []byte
	akeSendB, session.KeyLeft = gake.KexAkeSharedB(
		akeSendA,
		config.GetDecodedSecretKey(),
		config.GetDecodedPublicKey(msg.SenderID))

	fmt.Println("established shared key with left neighbor")

	msg = Message{
		MsgID:      uuid.New().String(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		MsgType:    MsgAkeSendB,
		ReceiverID: msg.SenderID,
		Content:    base64.StdEncoding.EncodeToString(akeSendB),
	}

	return msg
}

func EncryptAesGcm(plaintext string, session *Session) (string, error) {
	block, err := aes.NewCipher(session.SharedSecret[:])
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

func DecryptAesGcm(encryptedText string, session *Session) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(session.SharedSecret[:])
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
