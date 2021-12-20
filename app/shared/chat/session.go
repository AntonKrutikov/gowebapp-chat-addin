package chat

import (
	"encoding/json"
	"errors"
	"log"
	"sync"

	"github.com/nats-io/nats.go"
)

type Session struct {
	ID              string
	Subscriptions   map[string]*nats.Subscription
	SubscriptionMu  sync.Mutex
	Buffer          []*Message
	BufferAvailable chan bool
	BufferMu        sync.Mutex
	User            *User
	Rooms           map[string]*Room
	RoomsMu         sync.Mutex
}

var SessionStore = struct {
	Map map[string]*Session
	Mu  sync.Mutex
}{
	Map: map[string]*Session{},
	Mu:  sync.Mutex{},
}

func NewSession(id string, user *User) *Session {
	session := &Session{
		ID:              id,
		Subscriptions:   map[string]*nats.Subscription{},
		SubscriptionMu:  sync.Mutex{},
		Buffer:          []*Message{},
		BufferAvailable: make(chan bool, 1),
		BufferMu:        sync.Mutex{},
		User:            user,
		Rooms:           map[string]*Room{},
		RoomsMu:         sync.Mutex{},
	}

	return session
}

// TODO: simple validation of session token format or check for blocked
func ValidateSessionID(id string) bool {
	return true
}

func ValidateSubscriptionName(id string) bool {
	return true
}

func (s *Session) Subscribe(name string) (*nats.Subscription, error) {
	if !ValidateSubscriptionName(name) {
		return nil, errors.New("NATS(chat): Subscription name not valid")
	}
	messages := make(chan *nats.Msg) //TODO: buffered or not

	// Create NATS subscription
	sub, err := nc.ChanSubscribe(name, messages)
	if err != nil {
		log.Println("NATS: failed create subscription.", err)
		return nil, err
	}

	s.SubscriptionMu.Lock()
	defer s.SubscriptionMu.Unlock()
	if s.Subscriptions[name] == nil {
		s.Subscriptions[name] = sub
	} //else already subscribed

	// Start consumer in background
	go func(s *Session, pending chan *nats.Msg) {
		for delivery := range pending {
			m := Message{}
			err := json.Unmarshal(delivery.Data, &m)
			if err != nil {
				log.Println("NATS(chat): failed to decode message.", err)
			}
			log.Println(m)
			s.BufferMu.Lock()
			s.Buffer = append(s.Buffer, &m)
			s.BufferMu.Unlock()

			if len(s.BufferAvailable) == 0 {
				s.BufferAvailable <- true
			}
		}
	}(s, messages)

	return sub, nil
}

func (s *Session) Unsubscribe(name string) {
	s.SubscriptionMu.Lock()
	defer s.SubscriptionMu.Unlock()

	if s.Subscriptions[name] != nil {
		err := s.Subscriptions[name].Unsubscribe()
		if err != nil {
			log.Println("NATS: unsubscribe failed.", err)
		}
		delete(s.Subscriptions, name)
	}
}

func (s *Session) UnsubscribeAll() {
	s.SubscriptionMu.Lock()
	defer s.SubscriptionMu.Unlock()
	for name, sub := range s.Subscriptions {
		sub.Unsubscribe()
		delete(s.Subscriptions, name)
	}
}

func (s *Session) LeaveAllRooms() {
	rooms := make([]*Room, 0, len(s.Rooms))
	s.RoomsMu.Lock()
	for _, r := range s.Rooms {
		rooms = append(rooms, r)
	}
	s.RoomsMu.Unlock()

	for _, r := range rooms {
		r.Leave(s, true)
	}
}

func (s *Session) Publish(channel string, msg *Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		log.Println("NATS(chat): can't encode message.", err)
	}
	return nc.Publish(channel, body)
}
