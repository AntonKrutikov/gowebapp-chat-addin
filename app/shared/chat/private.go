package chat

import (
	"sync"
)

type PrivateHistory struct {
	User1   *User
	User2   *User
	History []*Message
	Mu      sync.Mutex
}

func (h *PrivateHistory) Add(m *Message) {
	h.Mu.Lock()
	if len(h.History) >= MAX_ROOM_HISTORY_MESSAGES {
		message := h.History[0]
		for _, attachments := range message.Attachments {
			AttachmentStore.Remove(attachments)
		}
		// Shift history
		h.History = h.History[1:]
	}
	h.History = append(h.History, m)
	h.Mu.Unlock()
}

type PrivateHistoryStoreType struct {
	History []*PrivateHistory
	Mu      sync.Mutex
}

var PrivateHistoryStore = &PrivateHistoryStoreType{
	History: []*PrivateHistory{},
	Mu:      sync.Mutex{},
}

func (store *PrivateHistoryStoreType) Get(u1 *User, u2 *User) *PrivateHistory {
	store.Mu.Lock()
	var h *PrivateHistory
	for _, history := range store.History {
		if (u1 == history.User1 && u2 == history.User2) || (u2 == history.User1 && u1 == history.User2) {
			h = history
			break
		}
	}
	if h == nil {
		h = &PrivateHistory{
			User1:   u1,
			User2:   u2,
			History: []*Message{},
			Mu:      sync.Mutex{},
		}
	}
	store.History = append(store.History, h)
	store.Mu.Unlock()

	return h
}
