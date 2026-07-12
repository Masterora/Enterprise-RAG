package documentupload

import (
	"mime/multipart"
	"net/http"
	"strings"
)

const MaxMemory = 32 << 20

type Input struct {
	SubjectID      string
	File           multipart.File
	Filename       string
	FileSize       int64
	ContentType    string
	ProcessingMode string
}

func ParseRequest(r *http.Request) (Input, error) {
	if err := r.ParseMultipartForm(MaxMemory); err != nil {
		return Input{}, err
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return Input{}, err
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
