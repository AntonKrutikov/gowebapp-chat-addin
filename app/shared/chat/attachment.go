package chat

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"mime/multipart"
	"os"
	"path"
	"sync"
	"time"

	"github.com/nfnt/resize"
)

type Attachment struct {
	ID           string    `json:"id"`
	Uploaded     time.Time `json:"uploaded"`
	Hash         string    `json:"hash"`
	OriginalPath string    `json:"original_url"`
	MinifiedPath string    `json:"minified_url"`
}

type AttachmentStoreType struct {
	List []*Attachment
	Mu   sync.Mutex
}

func (store *AttachmentStoreType) Add(a *Attachment) {
	store.Mu.Lock()
	store.List = append(store.List, a)
	store.Mu.Unlock()
}

func (store *AttachmentStoreType) Remove(a *Attachment) {
	store.Mu.Lock()
	temp := store.List[:0]
	last := true
	for _, attachment := range store.List {
		if a.ID != attachment.ID {
			temp = append(temp, attachment)
			if a.Hash == attachment.Hash {
				last = false
			}
		}
	}
	store.List = temp
	if last {
		a.RemoveFiles()
	}
	store.Mu.Unlock()
}

var AttachmentStore = &AttachmentStoreType{
	List: []*Attachment{},
	Mu:   sync.Mutex{},
}

func (a *Attachment) RemoveFiles() {
	os.Remove(a.OriginalPath)
	os.Remove(a.MinifiedPath)
}

func AttachmentUpload(dir string, fh *multipart.FileHeader) (*Attachment, error) {
	contentType := fh.Header.Get("Content-Type")
	extension := ""

	if contentType == "image/jpeg" {
		extension = "jpeg"
	} else if contentType == "image/png" {
		extension = "png"
	} else {
		return nil, errors.New("Unknown attachment mime/type")
	}

	file, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Calculate hash of file
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	// Reset file seek after hash calculation
	file.Seek(0, 0)

	hashSum := hash.Sum(nil)
	name := fmt.Sprintf("%s/%x.%s", dir, hashSum, extension)
	name_minified := fmt.Sprintf("%s/%x.min.jpeg", dir, hashSum)

	attachment := &Attachment{
		ID:           RandomString(32),
		Hash:         fmt.Sprintf("%x", hashSum),
		Uploaded:     time.Now(),
		OriginalPath: name,
		MinifiedPath: name_minified,
	}

	temp, err := os.Open(name)
	if err == nil {
		AttachmentStore.Add(attachment)
		return attachment, nil
	}

	// If file not found by hash - create it
	temp, err = os.Create(name)
	if err != nil {
		return nil, err
	}
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	_, err = temp.Write(bytes)
	if err != nil {
		return nil, err
	}

	// Create minified version, minified version always JPEG

	file.Seek(0, 0)
	minified, err := os.Create(name_minified)
	if err != nil {
		return nil, err
	}
	src, _, err := image.Decode(file)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Can't create image thumbnail. %s", err))
	}
	dst := resize.Resize(300, 0, src, resize.Lanczos3)

	jpeg.Encode(minified, dst, &jpeg.Options{Quality: 80})

	AttachmentStore.Add(attachment)
	return attachment, nil
}

// Delete all attachments from upload folder
func AttachmentsCleanup(dir string) error {
	folder, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, d := range folder {
		os.Remove(path.Join([]string{dir, d.Name()}...))
	}
	return nil
}
