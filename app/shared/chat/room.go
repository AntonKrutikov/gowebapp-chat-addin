package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type Room struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Users        map[string]*User    `json:"-"`
	UsersMu      sync.Mutex          `json:"-"`
	Sessions     map[string]*Session `json:"-"`
	SessionsMu   sync.Mutex          `json:"-"`
	Type         string              `json:"type"`
	MaxUsers     int                 `json:"max_users"`
	Permanent    bool                `json:"permanent"`
	LastActivity time.Time           `json:"-"`
}

type RoomHistory struct {
	History []*Message
	Mu      sync.Mutex
}

var RoomStore = struct {
	Map map[string]*Room
	Mu  sync.Mutex
}{
	Map: map[string]*Room{},
	Mu:  sync.Mutex{},
}

// Store room history by room.ID
var RoomHistoryStore = struct {
	Map map[string]*RoomHistory
	Mu  sync.Mutex
}{
	Map: map[string]*RoomHistory{},
	Mu:  sync.Mutex{},
}

type RoomSubscriptionError struct{}

func (re *RoomSubscriptionError) Error() string {
	return "Can't create NATS subscribtion. Possible bad room name."
}

type RoomAlreadyExistsError struct{}

func (re *RoomAlreadyExistsError) Error() string {
	return "Room already exists."
}

// Need to validate room name on creation (on types 'room.join', 'room.users' and 'room.leave')
func ValidateRoomName(room string) bool {
	// Max room length
	if MAX_ROOM_NAME_LENGTH > 0 {
		runeName := []rune(room)
		if len(runeName) > MAX_ROOM_NAME_LENGTH {
			return false
		}
	}
	//Exclude NATS wildcard symbols as '.' and '*'
	if strings.IndexRune(room, '.') != -1 || strings.IndexRune(room, '*') != -1 {
		return false
	}
	// Exclude system rooms
	if room == "chat.broadcast" {
		return false
	}

	if room == "" {
		return false
	}

	return true
}

func CreateRoom(name string, rtype string) (*Room, error) {
	RoomStore.Mu.Lock()
	defer RoomStore.Mu.Unlock()

	if RoomStore.Map[name] == nil {
		room := &Room{
			ID:           RandomString(32), // for public rooms - id same as name
			Name:         name,
			Users:        map[string]*User{},
			UsersMu:      sync.Mutex{},
			Sessions:     map[string]*Session{},
			SessionsMu:   sync.Mutex{},
			Type:         rtype,
			MaxUsers:     MAX_ROOM_USERS,
			Permanent:    false,
			LastActivity: time.Now(),
		}

		// try to create subscription in NATS
		temp := make(chan *nats.Msg)
		_, err := nc.ChanSubscribe(room.ID, temp)
		if err != nil {
			fmt.Println(err)
			return RoomStore.Map[name], &RoomSubscriptionError{}
		}

		if rtype == ROOM_PUBLIC {
			PublishMessage("chat.broadcast", MessageNewRoomCreated(room))
		}

		RoomStore.Map[name] = room
		return room, nil
	}

	return RoomStore.Map[name], &RoomAlreadyExistsError{}
}

func GetRoom(name string) (*Room, error) {
	RoomStore.Mu.Lock()
	defer RoomStore.Mu.Unlock()
	if room := RoomStore.Map[name]; room != nil {
		return room, nil
	}
	return nil, errors.New("Room not found")
}

func GetRoomByID(id string) (*Room, error) {
	RoomStore.Mu.Lock()
	defer RoomStore.Mu.Unlock()
	for _, room := range RoomStore.Map {
		if room.ID == id {
			return room, nil
		}
	}
	return nil, errors.New("Room not found")
}

func DeleteRoom(room *Room) {
	RoomStore.Mu.Lock()
	if RoomStore.Map[room.Name] != nil {
		delete(RoomStore.Map, room.Name)
		room = nil
	}
	RoomStore.Mu.Unlock()
}

// Active public rooms count
func RoomCount() int {
	RoomStore.Mu.Lock()
	defer RoomStore.Mu.Unlock()

	i := 0
	for _, r := range RoomStore.Map {
		if r.Type == "public" {
			i++
		}
	}
	return i
}

func (room *Room) UserCount() int {
	room.UsersMu.Lock()
	defer room.UsersMu.Unlock()

	return len(room.Users)
}

func JoinRoom(room *Room, session *Session, notify bool) error {

	room.SessionsMu.Lock()
	if room.Sessions[session.ID] == nil {
		_, err := session.Subscribe(room.ID)
		if err != nil {
			room.SessionsMu.Unlock()
			return err
		}
		room.Sessions[session.ID] = session
		session.Rooms[room.Name] = room
		PublishMessage(session.ID, MessageRoomJoinWithHistory(session, room, GetRoomHistory(room, session.User)))
	}
	room.SessionsMu.Unlock()

	room.UsersMu.Lock()
	if room.Users[session.User.ID] == nil {
		room.Users[session.User.ID] = session.User

		if notify {
			PublishMessage(room.ID, MessageRoomJoin(session, room))
		}
	}
	room.UsersMu.Unlock()
	return nil
}

func (room *Room) Leave(session *Session, notify bool) {
	room.SessionsMu.Lock()
	if room.Sessions[session.ID] != nil {
		session.Unsubscribe(room.ID)
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

	if last && user != nil {
		delete(room.Users, user.ID)
		if notify {
			response := MessageRoomLeave(session, room)
			body, _ := json.Marshal(response)
			nc.Publish(room.ID, body)
		}
	}
	room.UsersMu.Unlock()

	// If no users in room - free it name by deleting from RoomStore
	if len(room.Users) == 0 && room.Permanent == false {
		if room.Type == ROOM_PUBLIC {
			PublishMessage("chat.broadcast", MessageRoomDeleted(room))
		}
		DeleteRoom(room)
	}

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

func RoomExists(name string) bool {
	RoomStore.Mu.Lock()
	defer RoomStore.Mu.Unlock()

	if RoomStore.Map[name] != nil {
		return true
	}

	return false
}

func RoomExistsByID(id string) bool {
	RoomStore.Mu.Lock()
	defer RoomStore.Mu.Unlock()

	for _, room := range RoomStore.Map {
		if room.ID == id {
			return true
		}
	}

	return false
}

func (r *Room) HasUser(u *User) bool {
	r.UsersMu.Lock()
	defer r.UsersMu.Unlock()

	if r.Users[u.ID] != nil {
		return true
	}
	return false
}

func AddToHistory(room *Room, msg *Message) {
	roomHistory := &RoomHistory{}

	RoomHistoryStore.Mu.Lock()
	if RoomHistoryStore.Map[room.ID] != nil {
		roomHistory = RoomHistoryStore.Map[room.ID]
	} else {
		RoomHistoryStore.Map[room.ID] = roomHistory
	}
	RoomHistoryStore.Mu.Unlock()

	roomHistory.Mu.Lock()
	if len(roomHistory.History) >= MAX_ROOM_HISTORY_MESSAGES {
		roomHistory.History = roomHistory.History[1:]
	}
	roomHistory.History = append(roomHistory.History, msg)
	roomHistory.Mu.Unlock()
}

func GetRoomHistory(room *Room, for_user *User) []*Message {
	history := []*Message{}

	RoomHistoryStore.Mu.Lock()
	if roomHistory := RoomHistoryStore.Map[room.ID]; roomHistory != nil {
		roomHistory.Mu.Lock()
		for _, message := range roomHistory.History {
			// Filter message if it from user in MuteList
			from := GetUser(message.From.ID, "")
			muted, since := for_user.CheckInMute(from)
			if muted && since.Before(message.Timestamp) {
				continue
			}
			StripMissingAttachments(message)
			history = append(history, message)
		}
		roomHistory.Mu.Unlock()
	}
	RoomHistoryStore.Mu.Unlock()

	return history
}
