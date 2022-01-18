package chat

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type User struct {
	ID                   string              `json:"id"`
	Name                 string              `json:"name"`
	Sessions             map[string]*Session `json:"-"`
	SessionsMu           sync.Mutex          `json:"-"`
	FixedWindowCounter   int                 `json:"-"`
	FixedWindowCounterMu sync.Mutex          `json:"-"`
	MuteList             map[*User]time.Time `json:"-"`
	MuteListMu           sync.Mutex          `json:"-"`
	UploadBytes          int64               `json:"-"`
	UploadBytesMu        sync.Mutex          `json:"-"`
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
			ID:                   id,
			Name:                 name,
			Sessions:             map[string]*Session{},
			SessionsMu:           sync.Mutex{},
			FixedWindowCounter:   0,
			FixedWindowCounterMu: sync.Mutex{},
			MuteList:             map[*User]time.Time{},
			MuteListMu:           sync.Mutex{},
			UploadBytes:          0,
			UploadBytesMu:        sync.Mutex{},
		}

		go func() {
			ticker := time.NewTicker(FIXED_WINDOW_INTERVAL)
			for range ticker.C {
				user.FixedWindowCounterMu.Lock()
				user.FixedWindowCounter = 0
				user.FixedWindowCounterMu.Unlock()
			}
		}()

		// Reset upload quota
		go func() {
			ticker := time.NewTicker(UPLOAD_QUOTA_RESET_TIMEOUT)
			for range ticker.C {
				user.UploadBytesMu.Lock()
				user.UploadBytes = 0
				user.UploadBytesMu.Unlock()
			}
		}()
		UserStore.Map[id] = user
	}

	return UserStore.Map[id]
}

func UserExists(id string) bool {
	UserStore.Mu.Lock()
	defer UserStore.Mu.Unlock()

	if UserStore.Map[id] != nil {
		return true
	}

	return false
}

func (u *User) NewSession() *Session {
	id := u.ID + ":" + RandomString(32)

	session := NewSession(id, u)

	u.SessionsMu.Lock()
	u.Sessions[id] = session
	fmt.Println(len(u.Sessions))
	u.SessionsMu.Unlock()

	SessionStore.Mu.Lock()
	SessionStore.Map[id] = session
	// Create self subscription for this session and this user too
	if session.Subscriptions[id] == nil {
		session.Subscribe(id)
		session.Subscribe(u.ID)
		session.Subscribe("chat.broadcast")
	}
	SessionStore.Mu.Unlock()

	// HearBeat. Need to reset timer in GetSession
	session.TimeToDie = time.AfterFunc(HEART_BEAT_TIMEOUT, func() { u.DeleteSession(session) })

	return session
}

func (u *User) GetSession(id string) (*Session, error) {
	SessionStore.Mu.Lock()
	defer SessionStore.Mu.Unlock()

	session := SessionStore.Map[id]

	if session == nil {
		return session, errors.New("Session not found")
	}

	// HeartBeat.
	// If session will be not touched while timeout duration then it will be destroyed and next attempt to access it must return error (404 in controller for example)
	if !session.TimeToDie.Stop() {
		<-session.TimeToDie.C
	}
	session.TimeToDie.Reset(HEART_BEAT_TIMEOUT)

	return session, nil
}

func (u *User) DeleteSession(s *Session) {
	s.LeaveAllRooms()
	s.UnsubscribeAll()

	SessionStore.Mu.Lock()
	delete(SessionStore.Map, s.ID)
	SessionStore.Mu.Unlock()

	u.SessionsMu.Lock()
	delete(u.Sessions, s.ID)
	u.SessionsMu.Unlock()

	s.Closed <- true

	s = nil
}

func (u *User) AddToMute(target *User) {
	u.MuteListMu.Lock()
	u.MuteList[target] = time.Now()
	u.MuteListMu.Unlock()
}

func (u *User) RemoveFromMute(target *User) {
	u.MuteListMu.Lock()
	delete(u.MuteList, target)
	u.MuteListMu.Unlock()
}

func (u *User) CheckInMute(target *User) (bool, time.Time) {
	muted := false
	since := time.Time{}
	u.MuteListMu.Lock()
	since, muted = u.MuteList[target]
	u.MuteListMu.Unlock()
	return muted, since
}
