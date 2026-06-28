//go:build e2e

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type uploadProfilesResponse struct {
	Profiles []uploadProfile `json:"profiles"`
}

type uploadProfile struct {
	Name              string   `json:"name"`
	MaxSize           int64    `json:"maxSize"`
	TTLSeconds        int64    `json:"ttlSeconds"`
	AllowedExtensions []string `json:"allowedExtensions"`
	AllowedMimeTypes  []string `json:"allowedMimeTypes"`
	TusRequired       bool     `json:"tusRequired"`
	MaxCount          int32    `json:"maxCount"`
	InstantEnabled    bool     `json:"instantEnabled"`
}

type multipartUploadResponse struct {
	Files []uploadedFile `json:"files"`
}

type uploadedFile struct {
	Filename    string `json:"filename"`
	Originame   string `json:"originame"`
	Size        string `json:"size"`
	FileID      string `json:"fileId"`
	ReferenceID string `json:"referenceId"`
	Profile     string `json:"profile"`
	Status      string `json:"status"`
	ExpiresAt   string `json:"expiresAt"`
	URL         string `json:"url"`
}

func TestUploadAndCDNFileDownload(t *testing.T) {
	requireEnv(t)

	admin := e2e.loginAsAdmin(t)

	profilesResp := e2e.getRaw(t, "/upload/profiles", admin.Token, nil)
	if profilesResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /upload/profiles status = %d body = %s", profilesResp.StatusCode, string(profilesResp.Body))
	}
	var profiles uploadProfilesResponse
	if err := json.Unmarshal(profilesResp.Body, &profiles); err != nil {
		t.Fatalf("decode profiles: %v; body=%s", err, string(profilesResp.Body))
	}
	if !hasUploadProfile(profiles.Profiles, "document") {
		t.Fatalf("profiles missing document: %#v", profiles.Profiles)
	}

	body := []byte("hello cdn file")
	info := []map[string]string{{
		"name":        "e2e.txt",
		"size":        fmt.Sprintf("%d", len(body)),
		"profile":     "document",
		"contentType": "text/plain",
	}}
	infoJSON, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	uploadResp := e2e.postMultipart(t, "/upload", admin.Token, map[string]string{
		"json": string(infoJSON),
	}, map[string][]byte{
		"e2e.txt": body,
	})
	if uploadResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /upload status = %d body = %s", uploadResp.StatusCode, string(uploadResp.Body))
	}
	var uploaded multipartUploadResponse
	if err = json.Unmarshal(uploadResp.Body, &uploaded); err != nil {
		t.Fatalf("decode upload response: %v; body=%s", err, string(uploadResp.Body))
	}
	if len(uploaded.Files) != 1 {
		t.Fatalf("uploaded files = %#v, want one file", uploaded.Files)
	}
	file := uploaded.Files[0]
	if !isPublicReferenceID(file.ReferenceID) || file.URL == "" || !strings.HasPrefix(file.URL, "/cdn/file/"+file.ReferenceID) {
		t.Fatalf("uploaded file = %#v, want cdn file URL and public referenceId", file)
	}
	if uploadResponseExposesStorageKey(t, uploadResp.Body) || strings.HasPrefix(file.Filename, "files/") {
		t.Fatalf("uploaded file exposes storage key: %#v", file)
	}
	if isNumeric(file.ReferenceID) || isNumeric(strings.TrimPrefix(file.URL, "/cdn/file/")) {
		t.Fatalf("uploaded file exposes numeric reference id: %#v", file)
	}

	download := e2e.getRaw(t, file.URL, admin.Token, nil)
	if download.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d body = %s", file.URL, download.StatusCode, string(download.Body))
	}
	if string(download.Body) != string(body) {
		t.Fatalf("download body = %q, want %q", string(download.Body), string(body))
	}
	if got := download.Header.Get("Content-Disposition"); !strings.Contains(got, "attachment") || !strings.Contains(got, "e2e.txt") {
		t.Fatalf("Content-Disposition = %q, want attachment filename", got)
	}

	inline := e2e.getRaw(t, file.URL+"?display=inline", admin.Token, nil)
	if inline.StatusCode != http.StatusOK {
		t.Fatalf("GET inline status = %d body = %s", inline.StatusCode, string(inline.Body))
	}
	if got := inline.Header.Get("Content-Disposition"); !strings.Contains(got, "inline") {
		t.Fatalf("inline Content-Disposition = %q", got)
	}

	ranged := e2e.getRaw(t, file.URL, admin.Token, map[string]string{"Range": "bytes=0-4"})
	if ranged.StatusCode != http.StatusPartialContent {
		t.Fatalf("GET range status = %d body = %s", ranged.StatusCode, string(ranged.Body))
	}
	if string(ranged.Body) != "hello" {
		t.Fatalf("range body = %q, want hello", string(ranged.Body))
	}

	missing := e2e.getRaw(t, "/cdn/file/999999999999", admin.Token, nil)
	if missing.StatusCode != http.StatusBadRequest {
		t.Fatalf("numeric reference status = %d body = %s", missing.StatusCode, string(missing.Body))
	}
}

func TestUploadAvatarAndCDNImageProxy(t *testing.T) {
	requireEnv(t)

	admin := e2e.loginAsAdmin(t)
	body := mustDecodeBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/luzmkwAAAABJRU5ErkJggg==")
	info := []map[string]string{{
		"name":        "avatar.png",
		"size":        fmt.Sprintf("%d", len(body)),
		"profile":     "avatar",
		"contentType": "image/png",
	}}
	infoJSON, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	uploadResp := e2e.postMultipart(t, "/upload", admin.Token, map[string]string{
		"json": string(infoJSON),
	}, map[string][]byte{
		"avatar.png": body,
	})
	if uploadResp.StatusCode != http.StatusOK {
		t.Fatalf("POST avatar upload status = %d body = %s", uploadResp.StatusCode, string(uploadResp.Body))
	}
	var uploaded multipartUploadResponse
	if err = json.Unmarshal(uploadResp.Body, &uploaded); err != nil {
		t.Fatalf("decode avatar upload response: %v; body=%s", err, string(uploadResp.Body))
	}
	if len(uploaded.Files) != 1 {
		t.Fatalf("uploaded avatar files = %#v, want one file", uploaded.Files)
	}
	file := uploaded.Files[0]
	if !isPublicReferenceID(file.ReferenceID) || !strings.HasPrefix(file.URL, "/cdn/image/"+file.ReferenceID) {
		t.Fatalf("uploaded avatar = %#v, want cdn image URL and public referenceId", file)
	}
	if uploadResponseExposesStorageKey(t, uploadResp.Body) || strings.HasPrefix(file.Filename, "files/") {
		t.Fatalf("uploaded avatar exposes storage key: %#v", file)
	}

	e2e.postJSON(t, "/api/user.v1.CenterService/EditCenterAvatar", map[string]any{
		"referenceId": file.ReferenceID,
	}, admin.Token, &emptyResponse{})

	image := e2e.getRaw(t, file.URL+"/filters:format(webp)", "", nil)
	if image.StatusCode != http.StatusOK {
		t.Fatalf("GET image status = %d body = %s", image.StatusCode, string(image.Body))
	}
	if got := image.Header.Get("Content-Type"); !strings.HasPrefix(got, "image/webp") {
		t.Fatalf("image Content-Type = %q, want image/webp", got)
	}
	if !bytesHasWebPHeader(image.Body) {
		t.Fatalf("image body does not look like webp: len=%d prefix=%q", len(image.Body), firstBytes(string(image.Body), 16))
	}

	expired := e2e.getRaw(t, file.URL+"?expires=1&token=bad", "", nil)
	if expired.StatusCode != http.StatusForbidden {
		t.Fatalf("expired signature status = %d body = %s", expired.StatusCode, string(expired.Body))
	}
}

func hasUploadProfile(profiles []uploadProfile, name string) bool {
	for _, profile := range profiles {
		if profile.Name == name {
			return true
		}
	}
	return false
}

func uploadResponseExposesStorageKey(t *testing.T, body []byte) bool {
	t.Helper()
	var raw struct {
		Files []map[string]any `json:"files"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode raw upload response: %v; body=%s", err, string(body))
	}
	for _, file := range raw.Files {
		if _, ok := file["objectKey"]; ok {
			return true
		}
	}
	return false
}

func mustDecodeBase64(t *testing.T, value string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("decode test image: %v", err)
	}
	return data
}

func bytesHasWebPHeader(data []byte) bool {
	return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

func isPublicReferenceID(value string) bool {
	return strings.HasPrefix(value, "ref-") && !isNumeric(value)
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
