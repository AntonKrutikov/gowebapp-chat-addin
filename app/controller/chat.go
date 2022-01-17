package controller

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

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
	name := session.Values["username"].(string)

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
	name := session.Values["username"].(string)
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
		response := []*chat.Message{chat.MessageDisconnected()}
		body, _ := json.Marshal(response)
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
	name := session.Values["username"].(string)
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
	name := session.Values["username"].(string)
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
		return
	}

	chatSession.User.DeleteSession(chatSession)
	w.WriteHeader(200)
}

func ChatUploadPOST(w http.ResponseWriter, r *http.Request) {
	session := session.Instance(r)

	if session.Values["id"] == nil {
		w.WriteHeader(401)
		return
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(500)
		return
	}

	response := []string{}

	files := r.MultipartForm.File["files"]
	for _, f := range files {
		contentType := f.Header.Get("Content-Type")
		ftype := ""
		if contentType == "image/jpeg" {
			ftype = "jpeg"
		} else if contentType == "image/png" {
			ftype = "png"
		} else {
			continue
		}
		file, err := f.Open()
		defer file.Close()

		if err != nil {
			fmt.Println(err)
			continue
		}

		// Calculate hash of file. Try to not store duplicates
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			fmt.Println(err)
			continue
		}
		file.Seek(0, 0)

		fname := fmt.Sprintf("%s/%x.%s", chat.UPLOAD_DIR, hash.Sum(nil), ftype)

		// Check if file with hash exists, if yes - return it
		tempFile, err := os.Open(fmt.Sprintf(fname))
		if err == nil {
			response = append(response, tempFile.Name())
		} else {
			// Otherwise create new file and write upload data to it
			fmt.Println(err)
			tempFile, err = os.Create(fname)
			if err != nil {
				continue
			}
			fileBytes, err := ioutil.ReadAll(file)
			if err != nil {
				fmt.Println(err)
				continue
			}

			_, err = tempFile.Write(fileBytes)
			if err != nil {
				fmt.Println(err)
			}

			// Append to resulting slice of paths
			response = append(response, tempFile.Name())
		}
	}

	body, _ := json.Marshal(response)

	w.Write(body)

}
