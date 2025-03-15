package shared

import (
	"fmt"
)

type LeaderSession struct {
	transport Transport
	config    ConfigAccessor
	session   CryptoSession
}

func NewLeaderSession(transport Transport, config ConfigAccessor) *LeaderSession {
	s := &LeaderSession{
		transport: transport,
		session:   MakeSession(config),
		config:    config,
	}

	transport.SetMessageHandler(s.handleMessage)

	return s
}

func (s *LeaderSession) Init() {
	msg := GetAkeAMsg(&s.session, s.config)
	s.transport.Send(msg)
}

func (s *LeaderSession) GetKeyRef() *[32]byte {
	return &s.session.SharedSecret
}

func (s *LeaderSession) akeALeader(msg Message) {
	akeB, xi := HandleAkeA(msg, s.config, &s.session)
	s.transport.Send(akeB)
	if !xi.IsEmpty() {
		s.transport.Send(xi)
	}
}

func (s *LeaderSession) akeBLeader(msg Message) {
	xi := HandleAkeB(msg, s.config, &s.session)
	if !xi.IsEmpty() {
		s.transport.Send(xi)
	}
}

func (s *LeaderSession) xiLeader(msg Message) {
	HandleXi(msg, s.config, &s.session)
}

func (s *LeaderSession) handleMessage(msg Message) {
	switch msg.Type {
	case LeaderAkeAMsg:
		s.akeALeader(msg)
	case LeaderAkeBMsg:
		s.akeBLeader(msg)
	case LeaderXiMsg:
		s.xiLeader(msg)
	default:
		fmt.Printf("[ERROR] Unknown message type encountered\n")
	}
}
