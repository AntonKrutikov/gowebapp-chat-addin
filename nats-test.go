package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/nats-io/nats.go"
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

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func RandomStringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func RandomString(length int) string {
	return RandomStringWithCharset(length, charset)
}

func main() {
	// Connect to a server
	nc, err := nats.Connect(nats.DefaultURL)
	fmt.Println(err)

	chats := []string{"default", "dc", "marvel"}
	users := []string{"Batman", "Capitan America", "Hulk", "Iron Man", "Wonder Woman"}

	// Simple Publisher
	i := 0
	for i < 1000 {
		room := chats[rand.Intn(3)]
		msg := Message{
			Body:      fmt.Sprintf("Hi it's message №%d\n%s", i, RandomString(40)),
			Timestamp: time.Now(),
			To: MessageUser{
				Name: chats[rand.Intn(2)],
			},
			Type: "room.message",
			From: MessageUser{
				Name: users[rand.Intn(4)],
			},
		}
		body, _ := json.Marshal(&msg)
		err = nc.Publish(room, body)
		fmt.Println(err)
		nc.Flush()
		time.Sleep(time.Second / 4)
		i++
	}
}
