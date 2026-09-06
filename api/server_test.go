package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeRunner struct {
	input     string
	sessionID string
	result    string
	err       error
}

func (f *fakeRunner) Run(_ context.Context, input, sessionID string) (string, error) {
	f.input = input
	f.sessionID = sessionID
	return f.result, f.err
}

func TestInvokeRunsAgent(t *testing.T) {
	runner := &fakeRunner{result: "done"}
	request := httptest.NewRequest(http.MethodPost, "/agent/invoke", strings.NewReader(`{"input":"  hello  "}`))
	response := httptest.NewRecorder()

	Handler(runner).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if runner.input != "hello" {
		t.Errorf("agent input = %q, want %q", runner.input, "hello")
	}
	if runner.sessionID == "" {
		t.Error("agent session ID is empty")
	}
	assertResponse(t, response, invokeResponse{Success: true, Result: "done"})
}

func TestInvokeRejectsInvalidInput(t *testing.T) {
	for name, body := range map[string]string{
		"missing":  `{}`,
		"blank":    `{"input":"  "}`,
		"unknown":  `{"input":"hello","extra":true}`,
		"multiple": `{"input":"hello"}{"input":"again"}`,
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/agent/invoke", strings.NewReader(body))
			Handler(&fakeRunner{}).ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestInvokeReportsAgentFailureWithoutLeakingDetails(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/invoke", strings.NewReader(`{"input":"hello"}`))
	Handler(&fakeRunner{err: errors.New("provider secret")}).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if strings.Contains(response.Body.String(), "provider secret") {
		t.Error("response leaked the agent error")
	}
	assertResponse(t, response, invokeResponse{Success: false, Error: "agent execution failed"})
}

func TestInvokeOnlyAllowsPost(t *testing.T) {
	response := httptest.NewRecorder()
	Handler(&fakeRunner{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent/invoke", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
	if response.Header().Get("Allow") != http.MethodPost {
		t.Errorf("Allow = %q, want %q", response.Header().Get("Allow"), http.MethodPost)
	}
}

func assertResponse(t *testing.T, recorder *httptest.ResponseRecorder, want invokeResponse) {
	t.Helper()
	var got invokeResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got != want {
		t.Errorf("response = %+v, want %+v", got, want)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
}
