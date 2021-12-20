package chat

import (
	"sync"
)

type User struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Sessions   map[string]*Session `json:"-"`
	SessionsMu sync.Mutex          `json:"-"`
}

var UserStore = struct {
	Map map[string]*User
	Mu  sync.Mutex
}{
	Map: map[string]*User{},
	Mu:  sync.Mutex{},
}

func GetUser(id string, name string) *User {
	UserStore.Mu.Lock()
	defer UserStore.Mu.Unlock()

	if UserStore.Map[id] == nil {
		user := &User{
			ID:         id,
			Name:       name,
			Sessions:   map[string]*Session{},
			SessionsMu: sync.Mutex{},
		}
		UserStore.Map[id] = user
	}

	return UserStore.Map[id]
}

func (u *User) GetSession(id string) *Session {
	SessionStore.Mu.Lock()
	defer SessionStore.Mu.Unlock()

	if SessionStore.Map[id] == nil {
		SessionStore.Map[id] = NewSession(id, u)
	}

	u.SessionsMu.Lock()
	if u.Sessions[id] == nil {
		u.Sessions[id] = SessionStore.Map[id]
	}
	u.SessionsMu.Unlock()

	// Create self subscription by default to recieve direct messages and info messages
	if SessionStore.Map[id].Subscriptions[id] == nil {
		SessionStore.Map[id].Subscribe(id)
	}

	// Subscribe for message by client ID to for private
	if SessionStore.Map[id].Subscriptions[u.ID] == nil {
		SessionStore.Map[id].Subscribe(u.ID)
	}

	return SessionStore.Map[id]
}
