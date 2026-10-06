package jev

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/muse254/banterarena-engine/sentiment"
)

func fakeJev(t *testing.T, handler func(w http.ResponseWriter, req request)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		handler(w, req)
	}))
	t.Cleanup(srv.Close)
	return &Client{APIKey: "test-key", BaseURL: srv.URL}
}

func answer(w http.ResponseWriter, probs map[string]float64) {
	ans := map[string]any{}
	for k, v := range probs {
		ans[k] = map[string]any{"type": "noul", "noul": v}
	}
	json.NewEncoder(w).Encode(map[string]any{"model": "jev-1.13.0", "answers": ans})
}

func TestAskSendsNoulQuestionsAndParsesAnswers(t *testing.T) {
	c := fakeJev(t, func(w http.ResponseWriter, req request) {
		if req.Model != DefaultModel || req.State != "s" {
			t.Errorf("bad request: %+v", req)
		}
		if q := req.Questions["landed"]; q.Type != "noul" || q.Instructions != "did it land?" {
			t.Errorf("bad question: %+v", q)
		}
		answer(w, map[string]float64{"landed": 0.91})
	})
	p, err := c.Ask(context.Background(), "s", map[string]string{"landed": "did it land?"})
	if err != nil || p["landed"] != 0.91 {
		t.Fatalf("got %v, %v", p, err)
	}
}

func TestAskRetriesWhenRateLimited(t *testing.T) {
	var calls atomic.Int32
	c := fakeJev(t, func(w http.ResponseWriter, _ request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		answer(w, map[string]float64{"q": 0.2})
	})
	if _, err := c.Ask(context.Background(), "s", map[string]string{"q": "?"}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("expected 2 retries, got %d calls", calls.Load())
	}
}

func TestAskFailsFastOnBadKeyAndBadRequest(t *testing.T) {
	c := fakeJev(t, func(w http.ResponseWriter, _ request) { w.WriteHeader(http.StatusUnauthorized) })
	if _, err := c.Ask(context.Background(), "s", map[string]string{"q": "?"}); !errors.Is(err, ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
	c = fakeJev(t, func(w http.ResponseWriter, _ request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"detail":"bad"}`))
	})
	if _, err := c.Ask(context.Background(), "s", map[string]string{"q": "?"}); err == nil {
		t.Fatal("422 must be an error")
	}
	c = fakeJev(t, func(w http.ResponseWriter, _ request) { answer(w, map[string]float64{"other": 1}) })
	if _, err := c.Ask(context.Background(), "s", map[string]string{"q": "?"}); err == nil {
		t.Fatal("a missing answer must be an error")
	}
}

func TestSentimentAsksWithJokeContext(t *testing.T) {
	c := fakeJev(t, func(w http.ResponseWriter, req request) {
		if req.State != State("the joke", "the reply") {
			t.Errorf("state lost the joke context: %q", req.State)
		}
		answer(w, map[string]float64{qLanded: 0.88, qFlopped: 0.05})
	})
	preds, err := Sentiment{Client: c}.Predict(context.Background(), "the joke", []string{"the reply"})
	if err != nil || preds[0].Label != sentiment.Landed || preds[0].Confidence != 0.88 {
		t.Fatalf("got %+v, %v", preds, err)
	}
}

func TestFromProbabilities(t *testing.T) {
	cases := []struct {
		l, f float64
		want sentiment.Label
		conf float64
	}{
		{0.9, 0.1, sentiment.Landed, 0.9},
		{0.1, 0.8, sentiment.Flopped, 0.8},
		{0.2, 0.1, sentiment.Neutral, 0.8},
		{0.6, 0.7, sentiment.Flopped, 0.7}, // both yes: the stronger wins
	}
	for _, c := range cases {
		got := FromProbabilities(c.l, c.f)
		if got.Label != c.want || got.Confidence != c.conf {
			t.Errorf("FromProbabilities(%v,%v) = %+v; want %v @ %v", c.l, c.f, got, c.want, c.conf)
		}
	}
}
