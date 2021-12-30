package chat

import (
	"encoding/json"
	"log"
	"regexp"
	"time"

	"github.com/nats-io/nats.go"
)

var nc *nats.Conn

const HEART_BEAT_TIMEOUT = 30 * time.Second

// Restrict message count/per interval
const FIXED_WINDOW_MAX = 20
const FIXED_WINDOW_INTERVAL = 10 * time.Second

// Body of message type = 'room.message' will be truncated to this limit
// 0 - unlimited
const MAX_TEXT_MESSAGE_LENGTH = 0

// Private rooms named as user1.UserID() + ":" + user2.UserID + ":" + str(10). UserID is 12bytes in hex representation. 64 is enough, 128 more than enough.
const MAX_ROOM_NAME_LENGTH = 128

// Maximum count of users (not Sessions in single room). After that - new client will recieve 'room.overfull' message when try to join
// 0 - unlimited
const MAX_ROOM_USERS = 100

// Maximum number of rooms. If exceeded - clietn will recieve 'rooms.max_count' message when try to join to not exists room
// 0 - unlimited
const MAX_ROOM_COUNT = 100

// Maximum number of messages in room history. After that first item from slice will be truncated.
const MAX_ROOM_HISTORY_MESSAGES = 4

const ROOM_PUBLIC = "public"
const ROOM_PRIVATE = "private"

// Dictionary of bad words wich will be replaced by map value or **** if no map value
var BadWordsDictionary = map[*regexp.Regexp]string{
	regexp.MustCompile("fu+c+k"):             "f***",
	regexp.MustCompile(`http://[^\s]*`):      "--link-hide--",
	regexp.MustCompile(`telegram.me/[^\s]*`): "--telegram-hide--",
}

var DefaultRooms []*Room

func Init() {
	var err error
	nc, err = nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("NATS: failed connect, %v\n", err)
	}

	// Create Default Rooms
	defaultRoom, _ := CreateRoom("default", ROOM_PUBLIC)
	marvelRoom, _ := CreateRoom("marvel", ROOM_PUBLIC)
	dcRoom, _ := CreateRoom("dc", ROOM_PUBLIC)

	defaultRoom.Permanent = true
	marvelRoom.Permanent = true
	dcRoom.Permanent = true

	DefaultRooms = []*Room{defaultRoom, marvelRoom, dcRoom}
}

func PublishMessage(to string, msg *Message) error {
	body, _ := json.Marshal(msg)
	return nc.Publish(to, body)
}

func ProcessMessage(msg *Message, session *Session) {
	switch msg.Type {
	// Response to each heartbeat too, to avoid httphandler timeout to close session
	case "heartbeat":
		PublishMessage(session.ID, MessageHearbeat(session))
	// Return list of default rooms in response to 'rooms'
	case "rooms":
		PublishMessage(session.ID, MessageRoomList(session))
	// Assign client to existed or new room
	case "room.create":
		if !ValidateRoomName(msg.To.Name) {
			PublishMessage(session.ID, MessageRoomBadName(session))
			break
		}
		// If room limit is set - validate
		if MAX_ROOM_COUNT > 0 && RoomCount() >= MAX_ROOM_COUNT {
			PublishMessage(session.ID, MessageRoomsMaxCount(session))
			break
		}

		room, err := CreateRoom(msg.To.Name, ROOM_PUBLIC)
		if err != nil {
			if _, ok := err.(*RoomSubscriptionError); ok {
				PublishMessage(session.ID, MessageRoomBadName(session))
				break
			}
			if _, ok := err.(*RoomAlreadyExistsError); ok {
				PublishMessage(session.ID, MessageRoomAlreadyExists(session, room))
				break
			}
		}
	case "room.join":
		if !ValidateRoomName(msg.To.Name) {
			PublishMessage(session.ID, MessageRoomBadName(session))
			break
		}

		room, err := GetRoomByID(msg.To.ID)
		if err != nil {
			PublishMessage(session.ID, MessageRoomNotFound(session))
			break
		}

		// If user limit per room is set - validate
		if !room.HasUser(session.User) && room.MaxUsers > 0 && room.UserCount() >= MAX_ROOM_USERS {
			PublishMessage(session.ID, MessageRoomFull(session, room))
			break
		}

		err = JoinRoom(room, session, true)
		if err != nil {
			PublishMessage(session.ID, MessageRoomBadName(session))
			DeleteRoom(room)
		}
	// Remome client session from room
	case "room.leave":
		if !ValidateRoomName(msg.To.Name) {
			PublishMessage(session.ID, MessageRoomBadName(session))
			break
		}

		room, err := GetRoomByID(msg.To.ID)
		if err != nil {
			PublishMessage(session.ID, MessageRoomNotFound(session))
			break
		}

		if !room.HasUser(session.User) {
			PublishMessage(session.ID, MessageUserNotInRoom(session, room))
			break
		}
		room.Leave(session, true)
	// Return list of user in room
	case "room.users":
		if !RoomExistsByID(msg.To.ID) {
			PublishMessage(session.ID, MessageRoomNotFound(session))
			break
		}

		room, err := GetRoomByID(msg.To.ID)
		if err != nil {
			PublishMessage(session.ID, MessageRoomNotFound(session))
			break
		}

		if !room.HasUser(session.User) {
			PublishMessage(session.ID, MessageUserNotInRoom(session, room))
			break
		}

		PublishMessage(session.ID, MessageRoomUsers(session, room))
	// Process text messages inside room
	//
	case "room.message":
		if !ValidateRoomName(msg.To.Name) {
			PublishMessage(session.ID, MessageRoomBadName(session))
			break
		}

		room, err := GetRoomByID(msg.To.ID)
		if err != nil {
			PublishMessage(session.ID, MessageRoomNotFound(session))
			break
		}

		if !room.HasUser(session.User) {
			PublishMessage(session.ID, MessageUserNotInRoom(session, room))
			break
		}

		// Force set message from to session data, to prevent message fake
		msg.From = MessageUser{
			ID:   session.User.ID,
			Name: session.User.Name,
		}

		// Set timestamp on message before route to room
		msg.Timestamp = time.Now()

		if !ValidateMessage(msg, session) {
			break
		}

		// Check user spam to fast
		session.User.FixedWindowCounterMu.Lock()
		if session.User.FixedWindowCounter <= FIXED_WINDOW_MAX {
			session.User.FixedWindowCounter++
			PublishMessage(room.ID, msg)
			// save in history
			AddToHistory(room, msg)
		} else {
			PublishMessage(session.ID, MessageToManyRequests(session, msg.To))
		}
		session.User.FixedWindowCounterMu.Unlock()
	case "private.message":
		if !UserExists(msg.To.ID) {
			PublishMessage(session.ID, MessageUserNotFound(session, msg.To))
			break
		}
		if !ValidateMessage(msg, session) {
			break
		}
		// Force set message from to session data, to prevent message fake
		msg.From = MessageUser{
			ID:   session.User.ID,
			Name: session.User.Name,
		}

		// Set timestamp on message before route to room
		msg.Timestamp = time.Now()

		// Check user spam to fast
		session.User.FixedWindowCounterMu.Lock()
		if session.User.FixedWindowCounter <= FIXED_WINDOW_MAX {
			session.User.FixedWindowCounter++
			PublishMessage(msg.To.ID, msg)
			PublishMessage(session.User.ID, MessagePrivateDelivered(session, msg.Body, msg.To))
		} else {
			PublishMessage(session.ID, MessageToManyRequests(session, msg.To))
		}
		session.User.FixedWindowCounterMu.Unlock()
	}
}
