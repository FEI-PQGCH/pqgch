package shared

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"pqgch-client/gake"

	"github.com/google/uuid"
)

type Session struct {
	TkRight      []byte
	EskaRight    []byte
	KeyLeft      [32]byte
	KeyRight     [32]byte
	Xs           [][32]byte
	Commitments  []gake.Commitment
	Coins        [][44]byte
	SharedSecret [64]byte
	OnSharedKey  func()
}

func MakeSession(config ConfigAccessor) Session {
	return Session{
		Xs:          make([][32]byte, len(config.GetNamesOrAddrs())),
		OnSharedKey: func() {},
		Commitments: make([]gake.Commitment, len(config.GetNamesOrAddrs())),
		Coins:       make([][44]byte, len(config.GetNamesOrAddrs())),
	}
}

func checkLeftRightKeys(session *Session, config ConfigAccessor) Message {
	if session.KeyRight != [32]byte{} && session.KeyLeft != [32]byte{} {
		fmt.Println("CRYPTO: established shared keys with both neighbors")
		xi := getXiMsg(session, config)
		checkXs(session, config)
		return xi
	}

	return Message{}
}

func getXiMsg(session *Session, config ConfigAccessor) Message {
	xi, coin, commitment := gake.ComputeXsCommitment(
		config.GetIndex(),
		session.KeyRight,
		session.KeyLeft,
		config.GetDecodedPublicKey(config.GetIndex()))

	session.Xs[config.GetIndex()] = xi
	session.Commitments[config.GetIndex()] = commitment
	session.Coins[config.GetIndex()] = coin

	var buffer bytes.Buffer
	buffer.Grow(32 + 1088 + 36 + 16 + 44)
	buffer.Write(xi[:])
	buffer.Write(commitment.CipherTextKem[:])
	buffer.Write(commitment.CipherTextDem[:])
	buffer.Write(commitment.Tag[:])
	buffer.Write(coin[:])

	msg := Message{
		ID:         uuid.New().String(),
		SenderID:   config.GetIndex(),
		SenderName: config.GetName(),
		Type:       config.GetMessageType(XiMsg),
		Content:    base64.StdEncoding.EncodeToString(buffer.Bytes()),
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

	ok := gake.CheckXs(session.Xs, len(config.GetNamesOrAddrs()))
	if ok {
		fmt.Println("CRYPTO: Xs check: success")
	} else {
		fmt.Println("CRYPTO: Xs check: fail")
		os.Exit(1)
	}

	ok = gake.CheckCommitments(len(config.GetNamesOrAddrs()), session.Xs, config.GetDecodedPublicKeys(), session.Coins, session.Commitments)
	if ok {
		fmt.Println("CRYPTO: Commitments check: success")
	} else {
		fmt.Println("CRYPTO: Commitments check: fail")
		os.Exit(1)
	}

	for i := 0; i < len(session.Xs); i++ {
		fmt.Printf("CRYPTO: X%d: %02x\n", i, (session.Xs)[i][:4])
	}

	pids := make([][20]byte, len(config.GetNamesOrAddrs()))
	stringPids := config.GetNamesOrAddrs()
	for i := 0; i < len(config.GetNamesOrAddrs()); i++ {
		var byteArr [20]byte
		copy(byteArr[:], []byte(stringPids[i]))
		pids[i] = byteArr
	}

	masterKey := gake.ComputeMasterKey(len(config.GetNamesOrAddrs()), config.GetIndex(), session.KeyLeft, session.Xs)
	skSid := gake.ComputeSkSid(len(config.GetNamesOrAddrs()), masterKey, pids)
	fmt.Printf("CRYPTO: SkSid%d: %02x...\n", config.GetIndex(), skSid[:4])

	copy(session.SharedSecret[:], skSid[:64])
	session.OnSharedKey()
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

func EncryptAesGcm(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
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

func EncryptAndHMAC(clusterSession *Session, mainSession *Session, config ConfigAccessor) Message {
	ciphertext := make([]byte, 32)

	for i := 0; i < 32; i++ {
		ciphertext[i] = mainSession.SharedSecret[i] ^ clusterSession.SharedSecret[i]
	}
	mac := hmac.New(sha256.New, clusterSession.SharedSecret[32:])
	mac.Write(ciphertext)
	tag := mac.Sum(nil)
	ciphertext = append(ciphertext, tag...)

	msg := Message{
		ID:         uuid.New().String(),
		SenderID:   -1,
		SenderName: config.GetName(),
		Type:       KeyMsg,
		Content:    base64.StdEncoding.EncodeToString(ciphertext),
	}

	return msg
}

func DecryptAndCheckHMAC(encryptedText []byte, session *Session) ([]byte, error) {
	ciphertext := encryptedText[:32]
	tag := encryptedText[32:]

	mac := hmac.New(sha256.New, session.SharedSecret[32:])
	mac.Write(ciphertext)
	expectedTag := mac.Sum(nil)

	if !hmac.Equal(tag, expectedTag) {
		fmt.Println("error: key message tag mismatch")
		return nil, errors.New("tag mismatch")
	}

	key := make([]byte, 32)

	for i := 0; i < 32; i++ {
		key[i] = session.SharedSecret[i] ^ ciphertext[i]
	}

	return key, nil
}

func HandleAkeA(
	msg Message,
	config ConfigAccessor,
	session *Session,
) (Message, Message) {
	akeB := getAkeBMsg(session, msg, config)
	xi := checkLeftRightKeys(session, config)
	return akeB, xi
}

func HandleAkeB(
	msg Message,
	config ConfigAccessor,
	session *Session,
) Message {
	akeSendB, _ := base64.StdEncoding.DecodeString(msg.Content)
	session.KeyRight = gake.KexAkeSharedA(akeSendB, session.TkRight, session.EskaRight, config.GetDecodedSecretKey())
	xi := checkLeftRightKeys(session, config)
	return xi
}

func HandleXi(
	msg Message,
	config ConfigAccessor,
	session *Session,
) {
	if msg.SenderID == config.GetIndex() {
		return
	}

	decoded, _ := base64.StdEncoding.DecodeString(msg.Content)

	xi := decoded[:32]
	kem := decoded[32 : 32+1088]
	dem := decoded[32+1088 : 32+1088+36]
	tag := decoded[32+1088+36 : 32+1088+36+16]
	coin := decoded[32+1088+36+16 : 32+1088+36+16+44]

	var xiArr [32]byte
	copy(xiArr[:], xi)
	var kemArr [1088]byte
	copy(kemArr[:], kem)
	var demArr [36]byte
	copy(demArr[:], dem)
	var tagArr [16]byte
	copy(tagArr[:], tag)
	var coinArr [44]byte
	copy(coinArr[:], coin)

	var commitment gake.Commitment
	commitment.CipherTextDem = demArr
	commitment.CipherTextKem = kemArr
	commitment.Tag = tagArr

	session.Commitments[msg.SenderID] = commitment
	session.Coins[msg.SenderID] = coinArr
	session.Xs[msg.SenderID] = xiArr
	checkXs(session, config)
}
