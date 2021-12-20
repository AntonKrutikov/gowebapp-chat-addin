package chat

import (
	"encoding/json"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

var nc *nats.Conn

const ROOM_PREFIX = "room."

func Init() {
	var err error
	nc, err = nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Printf("NATS: failed connect, %v\n", err)
	}
}

var rooms = []string{
	ROOM_PREFIX + "default",
	ROOM_PREFIX + "marvel",
	ROOM_PREFIX + "dc",
}

func ProcessMessage(msg *Message, session *Session) {
	switch msg.Type {
	case "rooms":
		ResponseRooms(msg.From)
	case "room.join":
		if !ValidateRoomName(msg.Body) {
			break
		}
		room := GetRoom(msg.Body)
		JoinRoom(room, session, true)
		ResponceRoomUsers(msg.From, room)
	case "room.leave":
		if !ValidateRoomName(msg.Body) {
			break
		}
		room := GetRoom(msg.Body)
		room.Leave(session, true)
	case "message":
		msg.Timestamp = time.Now()
		body, _ := json.Marshal(msg)
		nc.Publish(msg.To, body)
	case "private":
		//TODO: better algorithm
		roomName := "private." + RandomString(32)
		room := GetRoom(roomName)
		room.Type = "private"
		JoinRoom(room, session, false) // self join

		message := Message{
			Timestamp: time.Now(),
			To:        UserStore.Map[msg.To].Name,
			From:      session.User.Name,
			Type:      "private",
			Body:      roomName,
		}
		body, _ := json.Marshal(message)
		nc.Publish(session.ID, body)
		UserStore.Mu.Lock()
		for _, s := range UserStore.Map[msg.To].Sessions {
			JoinRoom(room, s, false)
			nc.Publish(s.ID, body)
		}
		UserStore.Mu.Unlock()

	}
}

func ResponseRooms(to string) error {
	mbody, _ := json.Marshal(rooms)
	msg := Message{
		Timestamp: time.Now(),
		To:        to,
		Type:      "rooms",
		Body:      string(mbody),
	}
	body, _ := json.Marshal(msg)
	return nc.Publish(to, body)
}

func ResponceRoomUsers(to string, room *Room) error {
	mbody, _ := json.Marshal(room.GetUsers())
	msg := Message{
		Timestamp: time.Now(),
		To:        to,
		From:      room.Name,
		Type:      "room.users",
		Body:      string(mbody),
	}
	body, _ := json.Marshal(msg)
	return nc.Publish(to, body)
}
