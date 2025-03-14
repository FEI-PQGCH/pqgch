package shared

import (
	"fmt"
)

type LeaderDevSession struct {
	transport Transport
	config    ConfigAccessor
	session   Session
}

func NewLeaderDevSession(transport Transport, config ConfigAccessor) *LeaderDevSession {
	s := &LeaderDevSession{
		transport: transport,
		session:   MakeSession(config),
		config:    config,
	}

	transport.SetMessageHandler(s.handleMessage)

	return s
}

func (s *LeaderDevSession) Init() {
	msg := GetAkeAMsg(&s.session, s.config)
	s.transport.Send(msg)
}

func (s *LeaderDevSession) GetKeyRef() *[32]byte {
	return &s.session.SharedSecret
}

func (s *LeaderDevSession) akeALeader(msg Message) {
	akeB, xi := HandleAkeA(msg, s.config, &s.session)
	s.transport.Send(akeB)
	if !xi.IsEmpty() {
		s.transport.Send(xi)
	}
}

func (s *LeaderDevSession) akeBLeader(msg Message) {
	xi := HandleAkeB(msg, s.config, &s.session)
	if !xi.IsEmpty() {
		s.transport.Send(xi)
	}
}

func (s *LeaderDevSession) xiLeader(msg Message) {
	HandleXi(msg, s.config, &s.session)
}

func (s *LeaderDevSession) handleMessage(msg Message) {
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
