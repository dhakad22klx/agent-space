package providers

import (
	"context"
	"errors"
	"fmt"

	"github.com/joho/godotenv"
	"google.golang.org/genai"
)

// Gemini talks to the Gemini API through the official genai SDK.
type Gemini struct {
	client *genai.Client
	model  string
}

// NewGemini builds a provider from the GEMINI_API_KEY and GEMINI_MODEL
// entries in .env.
func NewGemini(ctx context.Context) (*Gemini, error) {
	env, err := godotenv.Read(".env")
	if err != nil {
		return nil, fmt.Errorf("read .env: %w", err)
	}

	apiKey := env["GEMINI_API_KEY"]
	if apiKey == "" {
		return nil, errors.New("GEMINI_API_KEY is not set in .env")
	}

	model := env["GEMINI_MODEL"]
	if model == "" {
		return nil, errors.New("GEMINI_MODEL is not set in .env")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create gemini client: %w", err)
	}

	return &Gemini{client: client, model: model}, nil
}

// Model reports which model the provider is talking to.
func (g *Gemini) Model() string { return g.model }

// Generate sends input to the model and returns the generated text.
func (g *Gemini) Generate(ctx context.Context, input string) (string, error) {
	res, err := g.client.Models.GenerateContent(ctx, g.model, genai.Text(input), nil)
	if err != nil {
		return "", err
	}

	return res.Text(), nil
}
