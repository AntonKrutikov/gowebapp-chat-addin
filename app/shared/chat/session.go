package chat

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

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
	TimeToDie       *time.Timer
	Closed          chan bool
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
		Closed:          make(chan bool, 1),
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
	// TODO: Validate name
	if !ValidateSubscriptionName(name) {
		return nil, errors.New("NATS(chat): Subscription name not valid")
	}

	// Check if already subscribed on this channel, only 1 subscription per session needed
	s.SubscriptionMu.Lock()
	sub := s.Subscriptions[name]
	s.SubscriptionMu.Unlock()
	if sub != nil {
		log.Printf("NATS: session %s already subscribed to %s, returned existed subscription.\n", s.ID, name)
		return sub, nil
	}

	messages := make(chan *nats.Msg) //TODO: buffered or not

	// Create NATS subscription
	sub, err := nc.ChanSubscribe(name, messages)
	if err != nil {
		log.Println("NATS: failed create subscription.", err)
		return nil, err
	}

	// Start consumer in background
	go func(s *Session, pending chan *nats.Msg) {
		for delivery := range pending {
			m := Message{}
			err := json.Unmarshal(delivery.Data, &m)
			if err != nil {
				log.Println("NATS(chat): failed to decode message.", err)
			}
			//Not add muted user messages to room messages
			from := GetUser(m.From.ID, "")
			muted, _ := s.User.CheckInMute(from)
			if muted && m.Type == "room.message" {
				continue
			}

			s.BufferMu.Lock()
			s.Buffer = append(s.Buffer, &m)
			s.BufferMu.Unlock()

			if len(s.BufferAvailable) == 0 {
				s.BufferAvailable <- true
			}
		}
	}(s, messages)

	s.SubscriptionMu.Lock()
	s.Subscriptions[name] = sub
	s.SubscriptionMu.Unlock()

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
		err := sub.Unsubscribe()
		if err != nil {
			log.Panicln("NATS(chat): failed to unsubscribe")
		}
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
