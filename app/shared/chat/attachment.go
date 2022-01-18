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

	"github.com/nfnt/resize"
)

type Attachment struct {
	OriginalUrl string `json:"original_url"`
	MinifiedUrl string `json:"minified_url"`
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

	temp, err := os.Open(name)
	if err == nil {
		return &Attachment{OriginalUrl: name, MinifiedUrl: name_minified}, nil
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
		return nil, errors.New(fmt.Sprintf("Can't create imgae thumbnail. %s", err))
	}
	dst := resize.Resize(300, 0, src, resize.Lanczos3)

	jpeg.Encode(minified, dst, &jpeg.Options{Quality: 80})

	return &Attachment{OriginalUrl: name, MinifiedUrl: name_minified}, nil
}

// Delete all attachments from upload folder
func AttachmentsCleanup(dir string) error {
	folder, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, d := range folder {
		os.RemoveAll(path.Join([]string{dir, d.Name()}...))
	}
	return nil
}
