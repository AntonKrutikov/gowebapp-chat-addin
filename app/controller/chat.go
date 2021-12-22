package controller

import (
	"encoding/json"
	"fmt"
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
	chatSession := user.NewSession()

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
	chatSession, err := user.GetSession(chatSessionID)
	if err != nil {
		body, _ := json.Marshal(chat.MessageSessionNotFound())
		w.WriteHeader(404)
		w.Write(body)
		return
	}

	select {
	// Client break request
	case <-ctx.Done():
		chatSession.User.DeleteSession((chatSession))
	// Session not active
	case <-chatSession.Closed:
		body, _ := json.Marshal(chat.MessageDisconnected())
		w.WriteHeader(599)
		w.Write(body)
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
		fmt.Println(err)
		w.WriteHeader(500)
		w.Write([]byte("unknown message format"))
		return
	}

	user := chat.GetUser(id, name)
	chatSession, err := user.GetSession(chatSessionID)
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte(err.Error()))
		return
	}

	message.From = chat.MessageUser{
		ID:   chatSession.User.ID,
		Name: chatSession.User.Name,
	}

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
	chatSession, err := user.GetSession(chatSessionID)
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte(err.Error()))
	}

	chatSession.User.DeleteSession((chatSession))
	w.WriteHeader(200)
}
