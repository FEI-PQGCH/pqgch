package shared

import (
	"encoding/base64"
	"fmt"
	"pqgch-client/gake"
)

type ClusterSession struct {
	transport      Transport
	config         ConfigAccessor
	session        CryptoSession
	keyCiphertext  []byte
	mainSessionKey [32]byte
	Received       chan Message
}

func NewClusterSession(transport Transport, config ConfigAccessor) *ClusterSession {
	s := &ClusterSession{
		transport: transport,
		session:   MakeSession(config),
		config:    config,
		Received:  make(chan Message, 10),
	}

	s.session.OnSharedKey = func() {
		if s.keyCiphertext == nil {
			fmt.Println("[CRYPTO] No key ciphertext, skipping")
			return
		}
		s.getMasterKey(s.keyCiphertext)
	}

	transport.SetMessageHandler(s.handleMessage)

	return s
}

func NewClusterLeaderSession(transport Transport, config ConfigAccessor, keyRef *[32]byte) *ClusterSession {
	s := &ClusterSession{
		transport: transport,
		session:   MakeSession(config),
		config:    config,
	}

	s.session.OnSharedKey = func() {
		fmt.Println("[CRYPTO] Broadcasting master key to cluster")
		keyMsg := EncryptAndHMAC(keyRef[:], config, s.session.SharedSecret[:])
		s.transport.Send(keyMsg)
	}

	transport.SetMessageHandler(s.handleMessage)

	return s
}

func (s *ClusterSession) Init() {
	msg := GetAkeAMsg(&s.session, s.config)
	s.transport.Send(msg)
}

func (s *ClusterSession) akeA(msg Message) {
	akeB, xi := HandleAkeA(msg, s.config, &s.session)
	s.transport.Send(akeB)
	if !xi.IsEmpty() {
		s.transport.Send(xi)
	}
}

func (s *ClusterSession) akeB(msg Message) {
	xi := HandleAkeB(msg, s.config, &s.session)
	if !xi.IsEmpty() {
		s.transport.Send(xi)
	}
}

func (s *ClusterSession) xi(msg Message) {
	HandleXi(msg, s.config, &s.session)
}

func (s *ClusterSession) keyHandler(msg Message) {
	decodedContent, err := base64.StdEncoding.DecodeString(msg.Content)
	if err != nil {
		fmt.Printf("[ERROR] Failed to decode key message: %v\n", err)
		return
	}
	var wrappingKey [64]byte
	if s.session.SharedSecret == [gake.SsLen]byte{} {
		fmt.Println("[CRYPTO] GAKE handshake not complete; storing key message")
		s.keyCiphertext = decodedContent
		return
	}
	copy(wrappingKey[:], s.session.SharedSecret[:])
	recoveredKey, err := DecryptAndCheckHMAC(decodedContent, wrappingKey[:])
	if err != nil {
		fmt.Println("[ERROR] Failed decrypting key message:", err)
		return
	}
	copy(s.mainSessionKey[:], recoveredKey)
	fmt.Printf("[CRYPTO] Main session established: %02x\n", s.mainSessionKey[:4])
}

func (s *ClusterSession) getMasterKey(decodedContent []byte) {
	recoveredKey, err := DecryptAndCheckHMAC(decodedContent, s.session.SharedSecret[:])

	if err != nil {
		fmt.Println("[ERROR] Failed decrypting key message:", err)
		return
	}

	copy(s.mainSessionKey[:], recoveredKey)
	fmt.Printf("[CRYPTO] Main session established: %02x\n", s.mainSessionKey[:4])
}

func (s *ClusterSession) text(msg Message) {
	plainText, err := DecryptAesGcm(msg.Content, s.mainSessionKey[:])
	if err != nil {
		fmt.Println("[ERROR] Failed decrypting message:", err)
		return
	}
	msg.Content = plainText
	s.Received <- msg
}

func (s *ClusterSession) handleMessage(msg Message) {
	switch msg.Type {
	case AkeAMsg:
		s.akeA(msg)
	case AkeBMsg:
		s.akeB(msg)
	case XiMsg:
		s.xi(msg)
	case KeyMsg:
		s.keyHandler(msg)
	default:
		s.text(msg)
	}
}

func (s *ClusterSession) SendText(text string) {
	if [32]byte(s.mainSessionKey) == [32]byte{} {
		fmt.Println("[CRYPTO] No master key available, skipping")
		return
	}
	cipherText, err := EncryptAesGcm(text, s.mainSessionKey[:])
	if err != nil {
		fmt.Printf("[ERROR] Encryption failed: %v\n", err)
		return
	}
	msg := Message{
		ID:         GenerateUniqueID(),
		SenderID:   s.config.GetIndex(),
		SenderName: s.config.GetName(),
		Content:    cipherText,
		Type:       TextMsg,
		ClusterID:  s.config.GetIndex(),
	}
	s.transport.Send(msg)
}
