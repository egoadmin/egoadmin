//go:build e2e

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"
)

type rawResponse struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

type egoErrorResponse struct {
	Code    int32  `json:"code"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func (e *environment) postJSON(t *testing.T, path string, body any, token string, out any) {
	t.Helper()

	resp := e.postRaw(t, path, body, token)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("POST %s status = %d, body = %s", path, resp.StatusCode, string(resp.Body))
	}
	if errResp, ok := resp.egoError(); ok {
		t.Fatalf("POST %s returned error response: code=%d reason=%q message=%q body=%s", path, errResp.Code, errResp.Reason, errResp.Message, string(resp.Body))
	}
	if out == nil {
		return
	}
	if len(resp.Body) == 0 {
		t.Fatalf("POST %s returned empty body", path)
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		t.Fatalf("decode POST %s response: %v; body = %s", path, err, string(resp.Body))
	}
}

func (e *environment) postRaw(t *testing.T, path string, body any, token string) rawResponse {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request %s: %v", path, err)
	}
	req, err := http.NewRequest(http.MethodPost, e.httpURL(path), bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create request %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return e.doRaw(t, req)
}

func (e *environment) postMultipart(t *testing.T, path string, token string, fields map[string]string, files map[string][]byte) rawResponse {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatalf("write multipart field %s: %v", name, err)
		}
	}
	for filename, data := range files {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create multipart file %s: %v", filename, err)
		}
		if _, err = part.Write(data); err != nil {
			t.Fatalf("write multipart file %s: %v", filename, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, e.httpURL(path), &body)
	if err != nil {
		t.Fatalf("create multipart request %s: %v", path, err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return e.doRaw(t, req)
}

func (e *environment) getRaw(t *testing.T, path string, token string, headers map[string]string) rawResponse {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, e.httpURL(path), nil)
	if err != nil {
		t.Fatalf("create GET %s: %v", path, err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return e.doRaw(t, req)
}

func (e *environment) doRaw(t *testing.T, req *http.Request) rawResponse {
	t.Helper()

	resp, err := e.httpClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s %s response: %v", req.Method, req.URL.Path, err)
	}
	return rawResponse{StatusCode: resp.StatusCode, Body: data, Header: resp.Header.Clone()}
}

func (r rawResponse) egoError() (egoErrorResponse, bool) {
	var errResp egoErrorResponse
	if len(r.Body) == 0 {
		return errResp, false
	}
	if err := json.Unmarshal(r.Body, &errResp); err != nil {
		return egoErrorResponse{}, false
	}
	if errResp.Reason != "" || errResp.Message != "" {
		return errResp, true
	}
	return egoErrorResponse{}, false
}

func requireErrorResponse(t *testing.T, resp rawResponse, operation string) {
	t.Helper()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	if _, ok := resp.egoError(); ok {
		return
	}
	t.Fatalf("%s returned success response: status=%d body=%s", operation, resp.StatusCode, string(resp.Body))
}

func requireEgoErrorReason(t *testing.T, resp rawResponse, operation string, wantReason string) egoErrorResponse {
	t.Helper()
	errResp, ok := resp.egoError()
	if !ok {
		t.Fatalf("%s returned non-ego error response: status=%d body=%s", operation, resp.StatusCode, string(resp.Body))
	}
	if errResp.Reason != wantReason {
		t.Fatalf("%s reason = %q, want %q; message=%q body=%s", operation, errResp.Reason, wantReason, errResp.Message, string(resp.Body))
	}
	return errResp
}

func (e *environment) waitHTTPStatus(t *testing.T, path string, status int) {
	t.Helper()

	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, e.httpURL(path), nil)
		if err != nil {
			t.Fatalf("create GET %s: %v", path, err)
		}
		resp, err := e.httpClient.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == status {
				return
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("GET %s did not return %d: last error %v", path, status, lastErr)
}

func (e *environment) requireGETContains(t *testing.T, path string, fragment string) {
	t.Helper()

	resp, err := e.httpClient.Get(e.httpURL(path))
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read GET %s: %v", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, body = %s", path, resp.StatusCode, string(data))
	}
	if !strings.Contains(strings.ToLower(string(data)), strings.ToLower(fragment)) {
		t.Fatalf("GET %s body does not contain %q", path, fragment)
	}
}

func (e *environment) httpURL(path string) string {
	return httpURLForPort(e.ports.GatewayHTTP, path)
}

func httpURLForPort(port int, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
}
