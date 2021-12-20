package chat

import "time"

type Message struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Body      string    `json:"body"`
	From      string    `json:"from"`
	To        string    `json:"to"`
}

func ValidateMessage(msg *Message, s *Session) bool {
	return true
}
