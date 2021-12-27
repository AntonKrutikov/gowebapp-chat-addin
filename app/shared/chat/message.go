package chat

import (
	"encoding/json"
	"fmt"
	"strings"
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
	// Truncate message text if limit is set
	if msg.Type == "room.message" || msg.Type == "private.message" || msg.Type == "private.delivered" {
		if MAX_TEXT_MESSAGE_LENGTH > 0 {
			msg.Body = TruncateString(msg.Body, MAX_TEXT_MESSAGE_LENGTH)
		}

		// Simple bad words replacement
		for bad, good := range BadWordsDictionary {
			if good == "" {
				good = string(bad[0]) + strings.Repeat("*", len(bad)-1)
			}
			msg.Body = strings.ReplaceAll(msg.Body, bad, good)
		}
	}

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

func MessageRoomList(s *Session) *Message {
	list := []*Room{}
	RoomStore.Mu.Lock()
	for _, r := range RoomStore.Map {
		if r.Type == ROOM_PUBLIC {
			list = append(list, r)
		}
	}
	RoomStore.Mu.Unlock()

	rooms, _ := json.Marshal(list)

	return &Message{
		Timestamp: time.Now(),
		Type:      "room.list",
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
			ID:   room.ID,
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
			ID:   room.ID,
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
			ID:   room.ID,
			Name: room.Name,
		},
	}
}

func MessagePrivateCreated(s *Session, room *Room, calle *User) *Message {
	body, _ := json.Marshal(room)
	return &Message{
		Timestamp: time.Now(),
		Type:      "private.created",
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

func MessagePrivateInvite(s *Session, room *Room, calle *User) *Message {
	body, _ := json.Marshal(room)
	return &Message{
		Timestamp: time.Now(),
		Type:      "private.invite",
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

func MessageToManyRequests(s *Session, dest MessageUser) *Message {
	return &Message{
		Timestamp: time.Now(),
		Type:      "to_many_requests",
		Body:      "You are to fast in sending messages",
		To: MessageUser{
			ID:   s.User.ID,
			Name: s.User.Name,
		},
		From: dest,
	}
}

func MessageRoomBadName(s *Session) *Message {
	return &Message{
		Timestamp: time.Now(),
		Type:      "room.bad_name",
		Body:      "This room name is not valid or depricated",
		To: MessageUser{
			ID:   s.User.ID,
			Name: s.User.Name,
		},
	}
}

func MessageRoomFull(s *Session, room *Room) *Message {
	return &Message{
		Timestamp: time.Now(),
		Type:      "room.full",
		Body:      "This room is full",
		To: MessageUser{
			ID:   s.User.ID,
			Name: s.User.Name,
		},
		From: MessageUser{
			ID:   room.ID,
			Name: room.Name,
		},
	}
}

func MessageUserNotInRoom(s *Session, room *Room) *Message {
	return &Message{
		Timestamp: time.Now(),
		Type:      "room.not_joined",
		Body:      "You not join this room",
		To: MessageUser{
			ID:   s.User.ID,
			Name: s.User.Name,
		},
		From: MessageUser{
			ID:   room.ID,
			Name: room.Name,
		},
	}
}

func MessageRoomsMaxCount(s *Session) *Message {
	return &Message{
		Timestamp: time.Now(),
		Type:      "room.max_count",
		Body:      "Maximum number of rooms exceeded",
		To: MessageUser{
			ID:   s.User.ID,
			Name: s.User.Name,
		},
	}
}

func MessageRoomNotFound(s *Session) *Message {
	return &Message{
		Timestamp: time.Now(),
		Type:      "room.not_found",
		Body:      fmt.Sprintf("Room not found. Join or create first"),
		To: MessageUser{
			ID:   s.User.ID,
			Name: s.User.Name,
		},
	}
}

func MessageUserNotFound(s *Session, u MessageUser) *Message {
	return &Message{
		Timestamp: time.Now(),
		Type:      "user.not_found",
		Body:      fmt.Sprintf("User id:%s name:%s not in chat.", u.ID, u.Name),
		To: MessageUser{
			ID:   s.User.ID,
			Name: s.User.Name,
		},
	}
}

func MessageNewRoomCreated(room *Room) *Message {
	mbody, _ := json.Marshal(room)
	return &Message{
		Timestamp: time.Now(),
		Type:      "room.created",
		Body:      string(mbody),
		From: MessageUser{
			ID:   room.ID,
			Name: room.Name,
		},
	}
}

func MessageRoomAlreadyExists(s *Session, room *Room) *Message {
	return &Message{
		Timestamp: time.Now(),
		Type:      "room.already_exists",
		Body:      fmt.Sprintf("Room id:%s name:%s already exists. You can join it", room.ID, room.Name),
		To: MessageUser{
			ID:   s.User.ID,
			Name: s.User.Name,
		},
		From: MessageUser{
			ID:   room.ID,
			Name: room.Name,
		},
	}
}

func MessageRoomDeleted(room *Room) *Message {
	mbody, _ := json.Marshal(room)
	return &Message{
		Timestamp: time.Now(),
		Type:      "room.deleted",
		Body:      string(mbody),
		From: MessageUser{
			ID:   room.ID,
			Name: room.Name,
		},
	}
}

func MessageHearbeat(s *Session) *Message {
	return &Message{
		Timestamp: time.Now(),
		Type:      "heartbeat",
		Body:      "",
		To: MessageUser{
			ID:   s.ID,
			Name: s.User.Name,
		},
	}
}

func MessagePrivateDelivered(s *Session, body string, callee MessageUser) *Message {
	return &Message{
		Timestamp: time.Now(),
		Type:      "private.delivered",
		Body:      body,
		To: MessageUser{
			ID:   s.User.ID,
			Name: s.User.Name,
		},
		From: callee,
	}
}