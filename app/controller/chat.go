package controller

import (
	"encoding/json"
	"net/http"

	"github.com/josephspurrier/gowebapp/app/shared/chat"
	"github.com/josephspurrier/gowebapp/app/shared/session"
	"github.com/josephspurrier/gowebapp/app/shared/view"
)

// IndexGET displays the home page
func ChatGET(w http.ResponseWriter, r *http.Request) {
	// Get session
	session := session.Instance(r)

	if session.Values["id"] == nil {
		w.WriteHeader(401)
		return
	}

	v := view.New(r)
	v.Name = "chat"
	v.Render(w)
	return

}

func ChatJoinGET(w http.ResponseWriter, r *http.Request) {
	session := session.Instance(r)

	if session.Values["id"] == nil {
		w.WriteHeader(401)
		return
	}

	id := session.Values["id"].(string)
	name := session.Values["first_name"].(string)

	user := chat.GetUser(id, name)
	chatSession := user.GetSession(chat.RandomString(32))

	response := struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Session string `json:"session"`
	}{
		ID:      id,
		Name:    name,
		Session: chatSession.ID,
	}

	body, _ := json.Marshal(&response)

	w.Write(body)

}

func ChatUpdateGET(w http.ResponseWriter, r *http.Request) {
	session := session.Instance(r)
	ctx := r.Context()

	if session.Values["id"] == nil {
		w.WriteHeader(401)
		return
	}

	id := session.Values["id"].(string)
	name := session.Values["first_name"].(string)
	chatSessionID := r.URL.Query().Get("session")
	if chatSessionID == "" || !chat.ValidateSessionID(chatSessionID) {
		w.WriteHeader(401)
		return
	}

	user := chat.GetUser(id, name)
	chatSession := user.GetSession(chatSessionID)

	select {
	case <-ctx.Done():
		chatSession.LeaveAllRooms()
		chatSession.UnsubscribeAll()
	case <-chatSession.BufferAvailable:
		chatSession.BufferMu.Lock()
		body, _ := json.Marshal(chatSession.Buffer)
		chatSession.Buffer = nil
		chatSession.BufferMu.Unlock()

		w.Header().Add("Content-Type", "application/json")
		w.Write(body)
	}

}

func ChatSendPOST(w http.ResponseWriter, r *http.Request) {
	session := session.Instance(r)

	if session.Values["id"] == nil {
		w.WriteHeader(401)
		return
	}

	id := session.Values["id"].(string)
	name := session.Values["first_name"].(string)
	chatSessionID := r.URL.Query().Get("session")
	if chatSessionID == "" || !chat.ValidateSessionID(chatSessionID) {
		w.WriteHeader(401)
		return
	}

	message := chat.Message{}
	err := json.NewDecoder(r.Body).Decode(&message)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("unknown message format"))
		return
	}

	message.From = chatSessionID

	user := chat.GetUser(id, name)
	chatSession := user.GetSession(chatSessionID)

	if !chat.ValidateMessage(&message, chatSession) {
		w.WriteHeader(400)
		return
	}

	chat.ProcessMessage(&message, chatSession)

	w.WriteHeader(200)
	w.Write([]byte("{}"))
}

func ChatCloseGET(w http.ResponseWriter, r *http.Request) {
	session := session.Instance(r)

	if session.Values["id"] == nil {
		w.WriteHeader(401)
		return
	}

	id := session.Values["id"].(string)
	name := session.Values["first_name"].(string)
	chatSessionID := r.URL.Query().Get("session")
	if chatSessionID == "" || !chat.ValidateSessionID(chatSessionID) {
		w.WriteHeader(401)
		return
	}

	user := chat.GetUser(id, name)
	chatSession := user.GetSession(chatSessionID)

	chatSession.UnsubscribeAll()
	w.WriteHeader(200)
}

func ChatTestGET(w http.ResponseWriter, r *http.Request) {
	// id := r.URL.Query().Get("id")
	// session := chat.SessionStore[id]

	// // fmt.Println(chat.SessionStore, session)

	// for _, v := range session.Buffer {
	// 	fmt.Println(string(v.Data))
	// }
	// session.Buffer = nil
	// session.Subscriptions["foo"].Unsubscribe()
}
