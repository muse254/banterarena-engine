// Package jev calls TypeSafe's Jev model (https://typesafe.ai), a fast
// decision model that answers yes/no ("noul") questions with a calibrated
// probability. It's the one package in the engine that does network I/O, so
// the scoring rules stay pure and Jev stays swappable behind sentiment.Model.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	DefaultBaseURL = "https://api.typesafe.ai"
	DefaultModel   = "jev-latest"
	// EnvKey is the environment variable (and GitHub Actions secret) holding the API key.
	EnvKey = "TYPESAFE_API_KEY"
)

// Client talks to the systemone endpoint.
type Client struct {
	APIKey  string
	BaseURL string // DefaultBaseURL if empty
	Model   string // DefaultModel if empty
	HTTP    *http.Client
	// Retries is how many times a rate-limited (429) or overloaded (529)
	// request is retried, with backoff. Zero means 3.
	Retries int
}

// FromEnv builds a client from TYPESAFE_API_KEY, or errors if it isn't set.
func FromEnv() (*Client, error) {
	key := os.Getenv(EnvKey)
	if key == "" {
		return nil, fmt.Errorf("jev: %s is not set", EnvKey)
	}
	return &Client{APIKey: key}, nil
}

// Question is one yes/no question about the state.
type Question struct {
	Type         string `json:"type"` // always "noul" here
	Instructions string `json:"instructions"`
}

type request struct {
	State     string              `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]Question `json:"questions"`
}

type response struct {
	Model   string `json:"model"`
	Answers map[string]struct {
		Type string   `json:"type"`
		Noul *float64 `json:"noul"`
	} `json:"answers"`
}

// ErrAuth means the API key was missing or rejected; retrying won't help.
var ErrAuth = errors.New("jev: API key rejected (401)")

// Ask sends one state with yes/no questions and returns P(yes) per question name.
func (c *Client) Ask(ctx context.Context, state string, qs map[string]string) (map[string]float64, error) {
	req := request{State: state, Model: c.Model, Questions: map[string]Question{}}
	if req.Model == "" {
		req.Model = DefaultModel
	}
	for name, text := range qs {
		req.Questions[name] = Question{Type: "noul", Instructions: text}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	retries := c.Retries
	if retries == 0 {
		retries = 3
	}

	for attempt := 0; ; attempt++ {
		r, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/systemone", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		r.Header.Set("Authorization", "Bearer "+c.APIKey)
		r.Header.Set("Content-Type", "application/json")
		resp, err := hc.Do(r)
		if err != nil {
			return nil, fmt.Errorf("jev: %w", err)
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusOK:
			var out response
			if err := json.Unmarshal(raw, &out); err != nil {
				return nil, fmt.Errorf("jev: bad response: %w", err)
			}
			probs := make(map[string]float64, len(qs))
			for name := range qs {
				a, ok := out.Answers[name]
				if !ok || a.Noul == nil {
					return nil, fmt.Errorf("jev: no answer for %q", name)
				}
				probs[name] = *a.Noul
			}
			return probs, nil
		case resp.StatusCode == http.StatusUnauthorized:
			return nil, ErrAuth
		case (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 529) && attempt < retries:
			wait := time.Duration(250<<attempt) * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		default:
			return nil, fmt.Errorf("jev: HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(raw))
		}
	}
}
