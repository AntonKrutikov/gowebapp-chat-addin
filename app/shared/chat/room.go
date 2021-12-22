package chat

import (
	"encoding/json"
	"sync"
)

type Room struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Users      map[string]*User    `json:"-"`
	UsersMu    sync.Mutex          `json:"-"`
	Sessions   map[string]*Session `json:"-"`
	SessionsMu sync.Mutex          `json:"-"`
	Type       string              `json:"type"`
}

var RoomStore = struct {
	Map map[string]*Room
	Mu  sync.Mutex
}{
	Map: map[string]*Room{},
	Mu:  sync.Mutex{},
}

func GetRoom(name string) *Room {
	RoomStore.Mu.Lock()
	defer RoomStore.Mu.Unlock()

	if RoomStore.Map[name] == nil {
		RoomStore.Map[name] = &Room{
			ID:         name, // for public rooms - id same as name
			Name:       name,
			Users:      map[string]*User{},
			UsersMu:    sync.Mutex{},
			Sessions:   map[string]*Session{},
			SessionsMu: sync.Mutex{},
			Type:       "public",
		}
	}

	return RoomStore.Map[name]
}

func JoinRoom(room *Room, session *Session, notify bool) {

	room.SessionsMu.Lock()
	if room.Sessions[session.ID] == nil {
		room.Sessions[session.ID] = session
		session.Subscribe(room.Name)
		session.Rooms[room.Name] = room
	}
	room.SessionsMu.Unlock()

	room.UsersMu.Lock()
	if room.Users[session.User.ID] == nil {
		room.Users[session.User.ID] = session.User

		if notify {
			response := MessageRoomJoin(session, room)
			body, _ := json.Marshal(response)
			nc.Publish(room.Name, body)
		}
	}
	room.UsersMu.Unlock()
}

func (room *Room) Leave(session *Session, notify bool) {
	room.SessionsMu.Lock()
	if room.Sessions[session.ID] != nil {
		session.Unsubscribe(room.Name)
		delete(room.Sessions, session.ID)
		session.RoomsMu.Lock()
		delete(session.Rooms, room.Name)
		session.RoomsMu.Unlock()

	}
	room.SessionsMu.Unlock()

	room.UsersMu.Lock()
	last := true
	user := room.Users[session.User.ID]
	if user != nil {
		for s, _ := range user.Sessions {
			if room.Sessions[s] != nil {
				last = false
				break
			}
		}
	}

	if last {
		delete(room.Users, user.ID)
		if notify {
			response := MessageRoomLeave(session, room)
			body, _ := json.Marshal(response)
			nc.Publish(room.Name, body)
		}
	}
	room.UsersMu.Unlock()
}

func ValidateRoomName(room string) bool {
	return true
}

func (r *Room) GetUsers() []*User {
	r.UsersMu.Lock()
	defer r.UsersMu.Unlock()

	users := make([]*User, 0, len(r.Users))
	for _, u := range r.Users {
		users = append(users, u)
	}

	return users
}
