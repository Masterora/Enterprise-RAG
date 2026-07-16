package documentupload

import (
	"errors"
	"mime/multipart"
	"net/http"
	"strings"
)

const (
	MaxMemory      = 32 << 20
	MaxFileSize    = 50 << 20
	MaxRequestSize = MaxFileSize + (1 << 20)
)

var ErrFileTooLarge = errors.New("file exceeds the 50 MB limit")

type Input struct {
	SubjectID      string
	File           multipart.File
	Filename       string
	FileSize       int64
	ContentType    string
	ProcessingMode string
}

func ParseRequest(r *http.Request) (Input, error) {
	return parseRequest(r, MaxFileSize)
}

func parseRequest(r *http.Request, maxFileSize int64) (Input, error) {
	if err := r.ParseMultipartForm(MaxMemory); err != nil {
		cleanupMultipartForm(r)
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return Input{}, ErrFileTooLarge
		}
		return Input{}, err
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		cleanupMultipartForm(r)
		return Input{}, err
	}
	if header.Size > maxFileSize {
		_ = file.Close()
		cleanupMultipartForm(r)
		return Input{}, ErrFileTooLarge
	}

	return Input{
		SubjectID:      strings.TrimSpace(r.FormValue("subject_id")),
		File:           file,
		Filename:       header.Filename,
		FileSize:       header.Size,
		ContentType:    header.Header.Get("Content-Type"),
		ProcessingMode: strings.TrimSpace(r.FormValue("processing_mode")),
	}, nil
}

func cleanupMultipartForm(r *http.Request) {
	if r.MultipartForm != nil {
		_ = r.MultipartForm.RemoveAll()
	}
}
