package chat

import (
	"encoding/json"
	"log"
	"sort"
	"time"

	"github.com/nats-io/nats.go"
)

var nc *nats.Conn

const HEART_BEAT_TIMEOUT = 60 * time.Second

func Init() {
	var err error
	nc, err = nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Printf("NATS: failed connect, %v\n", err)
	}
}

var DefaultRooms = []*Room{
	GetRoom("default"),
	GetRoom("marvel"),
	GetRoom("dc"),
}

func ProcessMessage(msg *Message, session *Session) {
	switch msg.Type {
	case "rooms":
		response := MessageDefaultRooms(session)
		body, _ := json.Marshal(&response)
		nc.Publish(response.To.ID, body)
	case "room.join":
		if !ValidateRoomName(msg.Body) {
			break
		}
		room := GetRoom(msg.Body)
		JoinRoom(room, session, true)
	case "room.leave":
		if !ValidateRoomName(msg.Body) {
			break
		}
		room := GetRoom(msg.Body)
		room.Leave(session, true)
	case "room.users":
		if !ValidateRoomName(msg.Body) {
			break
		}
		room := GetRoom(msg.Body)
		body, _ := json.Marshal(MessageRoomUsers(session, room))
		nc.Publish(session.ID, body)
	case "room.message":
		//TODO: validation
		msg.Timestamp = time.Now()
		body, _ := json.Marshal(msg)
		nc.Publish(msg.To.Name, body)
	case "private":
		//TODO: check users exists
		//TODO: check to disallow overs join by room name
		users := []string{msg.From.ID, msg.To.ID}
		sort.Strings(users)
		roomName := users[0] + ":" + users[1]
		room := GetRoom(roomName)
		room.Type = "private"

		JoinRoom(room, session, false)
		user := GetUser(msg.To.ID, msg.To.Name)
		m := MessagePrivateInvite(session, room, user)
		m.Type = "private.created"
		body, _ := json.Marshal(m)
		nc.Publish(session.ID, body)

		// For callee join all sessions to room
		UserStore.Mu.Lock()
		for _, s := range UserStore.Map[msg.To.ID].Sessions {
			JoinRoom(room, s, false)
			m := MessagePrivateInvite(s, room, session.User)
			m.Type = "private.invite"
			body, _ := json.Marshal(m)
			nc.Publish(s.ID, body)
		}
		UserStore.Mu.Unlock()

	}
}
