package chat

import (
	"encoding/json"
	"time"
)

type Message struct {
	Timestamp time.Time   `json:"timestamp"`
	Type      string      `json:"type"`
	Body      string      `json:"body"`
	From      MessageUser `json:"from"`
	To        MessageUser `json:"to"`
}

type MessageUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func ValidateMessage(msg *Message, s *Session) bool {
	return true
}

func MessageDisconnected() *Message {
	return &Message{
		Timestamp: time.Now(),
		Type:      "disconnected",
		Body:      "Your session was disconnected",
	}
}

func MessageSessionNotFound() *Message {
	return &Message{
		Timestamp: time.Now(),
		Type:      "session.not_found",
		Body:      "Session with this ID not exists",
	}
}

func MessageDefaultRooms(s *Session) *Message {
	rooms, _ := json.Marshal(&DefaultRooms)
	return &Message{
		Timestamp: time.Now(),
		Type:      "rooms",
		Body:      string(rooms),
		To: MessageUser{
			ID:   s.ID,
			Name: s.User.Name,
		},
	}
}

func MessageRoomJoin(s *Session, room *Room) *Message {
	body, _ := json.Marshal(room)
	return &Message{
		Timestamp: time.Now(),
		Type:      "room.join",
		Body:      string(body),
		To: MessageUser{
			ID:   room.Name,
			Name: room.Name,
		},
		From: MessageUser{
			ID:   s.User.ID,
			Name: s.User.Name,
		},
	}
}

func MessageRoomLeave(s *Session, room *Room) *Message {
	body, _ := json.Marshal(room)
	return &Message{
		Timestamp: time.Now(),
		Type:      "room.leave",
		Body:      string(body),
		To: MessageUser{
			ID:   room.Name,
			Name: room.Name,
		},
		From: MessageUser{
			ID:   s.User.ID,
			Name: s.User.Name,
		},
	}
}

func MessageRoomUsers(s *Session, room *Room) *Message {
	users := room.GetUsers()
	body, _ := json.Marshal(users)

	return &Message{
		Timestamp: time.Now(),
		Type:      "room.users",
		Body:      string(body),
		To: MessageUser{
			ID:   s.User.ID,
			Name: s.User.Name,
		},
		From: MessageUser{
			ID:   room.Name,
			Name: room.Name,
		},
	}
}

func MessagePrivateInvite(s *Session, room *Room, calle *User) *Message {
	body, _ := json.Marshal(room)
	return &Message{
		Timestamp: time.Now(),
		Type:      "private",
		Body:      string(body),
		To: MessageUser{
			ID:   s.User.ID,
			Name: s.User.Name,
		},
		From: MessageUser{
			ID:   calle.ID,
			Name: calle.Name,
		},
	}
}
