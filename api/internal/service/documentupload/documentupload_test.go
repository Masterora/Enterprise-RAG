package documentupload

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseRequestAcceptsFileAtLimit(t *testing.T) {
	body, contentType := multipartRequestBody(t, []byte("test"))
	request := httptest.NewRequest(http.MethodPost, "/documents/upload", bytes.NewReader(body))
	request.Header.Set("Content-Type", contentType)

	input, err := parseRequest(request, 4)
	if err != nil {
		t.Fatalf("parse request: %v", err)
	}
	defer input.File.Close()
	defer request.MultipartForm.RemoveAll()

	if input.SubjectID != "subject-1" {
		t.Fatalf("subject id = %q, want subject-1", input.SubjectID)
	}
	if input.Filename != "document.txt" {
		t.Fatalf("filename = %q, want document.txt", input.Filename)
	}
	if input.FileSize != 4 {
		t.Fatalf("file size = %d, want 4", input.FileSize)
	}
}

func TestParseRequestRejectsOversizedFile(t *testing.T) {
	body, contentType := multipartRequestBody(t, []byte("large"))
	request := httptest.NewRequest(http.MethodPost, "/documents/upload", bytes.NewReader(body))
	request.Header.Set("Content-Type", contentType)

	_, err := parseRequest(request, 4)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("error = %v, want ErrFileTooLarge", err)
	}
}

func TestParseRequestMapsRequestBodyLimit(t *testing.T) {
	body, contentType := multipartRequestBody(t, []byte("test"))
	request := httptest.NewRequest(http.MethodPost, "/documents/upload", bytes.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	request.Body = http.MaxBytesReader(httptest.NewRecorder(), request.Body, int64(len(body)-1))

	_, err := parseRequest(request, 100)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("error = %v, want ErrFileTooLarge", err)
	}
}

func multipartRequestBody(t *testing.T, content []byte) ([]byte, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("subject_id", "subject-1"); err != nil {
		t.Fatalf("write subject id: %v", err)
	}
	part, err := writer.CreateFormFile("file", "document.txt")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return body.Bytes(), writer.FormDataContentType()
}
