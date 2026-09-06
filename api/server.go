// Package api exposes agent-space through a small HTTP API.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	agent "agent-harness/agent"
	providers "agent-harness/providers"

	"github.com/google/uuid"
)

const maxRequestBytes = 1 << 20

type agentRunner interface {
	Run(context.Context, string, string) (string, error)
}

type invokeRequest struct {
	Input string `json:"input"`
}

type invokeResponse struct {
	Success bool   `json:"success"`
	Result  string `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Handler returns the HTTP surface for an existing agent execution flow.
func Handler(runner agentRunner) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/agent/invoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, invokeResponse{Success: false, Error: "method not allowed"})
			return
		}

		var request invokeRequest
		body := http.MaxBytesReader(w, r.Body, maxRequestBytes)
		decoder := json.NewDecoder(body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, invokeResponse{Success: false, Error: "invalid request body"})
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, invokeResponse{Success: false, Error: "invalid request body"})
			return
		}
		request.Input = strings.TrimSpace(request.Input)
		if request.Input == "" {
			writeJSON(w, http.StatusBadRequest, invokeResponse{Success: false, Error: "input is required"})
			return
		}

		result, err := runner.Run(r.Context(), request.Input, uuid.NewString())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, invokeResponse{Success: false, Error: "agent execution failed"})
			return
		}

		writeJSON(w, http.StatusOK, invokeResponse{Success: true, Result: result})
	})
	return mux
}

// ListenAndServe creates the configured provider and serves the HTTP API.
func ListenAndServe(ctx context.Context, address string) error {
	provider, err := providers.NewGemini(ctx)
	if err != nil {
		return fmt.Errorf("configure agent: %w", err)
	}
	return http.ListenAndServe(address, Handler(agent.GetAgent(provider)))
}

func writeJSON(w http.ResponseWriter, status int, response invokeResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}
