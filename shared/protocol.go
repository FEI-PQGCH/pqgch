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

func MakeSession(config ConfigAccessor) Session {
	return Session{
		Xs: make([][32]byte, len(config.GetNamesOrAddrs())),
	}
}

func checkLeftRightKeys(session *Session, config ConfigAccessor) (bool, Message) {
	if session.KeyRight != [32]byte{} && session.KeyLeft != [32]byte{} {
		fmt.Println("CRYPTO: established shared keys with both neighbors")
		xi := getXiMsg(session, config)
		checkXs(session, config)
		return true, xi
	}
	return false, Message{}
}

func getXiMsg(session *Session, config ConfigAccessor) Message {
	xi, _, _ /* TODO: mi1, mi2 */ := gake.ComputeXsCommitment(
		config.GetIndex(),
		session.KeyRight,
		session.KeyLeft,
		config.GetDecodedPublicKey((config.GetIndex()+1)%len(config.GetNamesOrAddrs())))

	var xiArr [32]byte
	copy(xiArr[:], xi)

	(session.Xs)[config.GetIndex()] = xiArr
	msg := Message{
		ID:         uuid.New().String(),
		SenderID:   config.GetIndex(),
		SenderName: config.GetName(),
		Type:       config.GetMessageType(XiMsg),
		Content:    base64.StdEncoding.EncodeToString(xi[:]),
	}

	return msg
}

func checkXs(session *Session, config ConfigAccessor) {
	for i := 0; i < len(session.Xs); i++ {
		if (session.Xs)[i] == [32]byte{} {
			return
		}
	}

	fmt.Println("CRYPTO: received all Xs")
	for i := 0; i < len(session.Xs); i++ {
		fmt.Printf("CRYPTO: X%d: %02x\n", i, (session.Xs)[i][:4])
	}
	dummyPids := make([][20]byte, len(config.GetNamesOrAddrs())) // TODO: replace with actual pids

	masterKey := gake.ComputeMasterKey(len(config.GetNamesOrAddrs()), config.GetIndex(), session.KeyLeft, session.Xs)
	skSid := gake.ComputeSkSid(len(config.GetNamesOrAddrs()), masterKey, dummyPids)
	fmt.Printf("CRYPTO: SkSid%d: %02x...\n", config.GetIndex(), skSid[:4])

	copy(session.SharedSecret[:], skSid[:32])
}

func GetAkeAMsg(session *Session, config ConfigAccessor) Message {
	var rightIndex = (config.GetIndex() + 1) % len(config.GetNamesOrAddrs())
	var akeSendARight []byte
	akeSendARight, session.TkRight, session.EskaRight = gake.KexAkeInitA(config.GetDecodedPublicKey(rightIndex))

	msg := Message{
		ID:         uuid.New().String(),
		SenderID:   config.GetIndex(),
		SenderName: config.GetName(),
		Type:       config.GetMessageType(AkeAMsg),
		ReceiverID: rightIndex,
		Content:    base64.StdEncoding.EncodeToString(akeSendARight),
	}

	return msg
}

func getAkeBMsg(session *Session, msg Message, config ConfigAccessor) Message {
	akeSendA, _ := base64.StdEncoding.DecodeString(msg.Content)

	var akeSendB []byte
	akeSendB, session.KeyLeft = gake.KexAkeSharedB(
		akeSendA,
		config.GetDecodedSecretKey(),
		config.GetDecodedPublicKey(msg.SenderID))

	fmt.Println("CRYPTO: established shared key with left neighbor")

	msg = Message{
		ID:         uuid.New().String(),
		SenderID:   config.GetIndex(),
		SenderName: config.GetName(),
		Type:       config.GetMessageType(AkeBMsg),
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

// TODO: return new session instead of modifying the existing one

func HandleAkeA(
	msg Message,
	config ConfigAccessor,
	session *Session,
	send func(Message),
	broadcast func(Message),
) {
	fmt.Println("CRYPTO: received AKE A message")
	responseMsg := getAkeBMsg(session, msg, config)
	fmt.Println("CRYPTO: sending AKE B message")
	send(responseMsg)
	ok, xi := checkLeftRightKeys(session, config)

	if ok {
		fmt.Println("CRYPTO: sending Xi")
		broadcast(xi)
	}
}

func HandleAkeB(
	msg Message,
	config ConfigAccessor,
	session *Session,
	broadcast func(Message),
) {
	fmt.Println("CRYPTO: received AKE B message")
	akeSendB, _ := base64.StdEncoding.DecodeString(msg.Content)
	session.KeyRight = gake.KexAkeSharedA(akeSendB, session.TkRight, session.EskaRight, config.GetDecodedSecretKey())
	fmt.Println("CRYPTO: established shared key with right neighbor")
	ok, xi := checkLeftRightKeys(session, config)

	if ok {
		fmt.Println("CRYPTO: sending Xi")
		broadcast(xi)
	}
}

func HandleXi(
	msg Message,
	config ConfigAccessor,
	session *Session,
) {
	if msg.SenderID == config.GetIndex() {
		return
	}
	fmt.Println("CRYPTO: received Xi")
	xi, _ := base64.StdEncoding.DecodeString(msg.Content)
	var xiArr [32]byte
	copy(xiArr[:], xi)
	session.Xs[msg.SenderID] = xiArr
	checkXs(session, config)
}
