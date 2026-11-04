package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// wireServer answers like the Messages API and records what the real SDK sent.
func wireServer(t *testing.T, status int, reply string, got *map[string]any, key *string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, got)
		*key = r.Header.Get("X-Api-Key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ANTHROPIC_BASE_URL", srv.URL)
}

func TestAnthropicSendRequestAndReplyOnTheWire(t *testing.T) {
	var got map[string]any
	var key string
	wireServer(t, 200, `{"id":"m","type":"message","role":"assistant","model":"claude-haiku-5-5","stop_reason":"end_turn",
		"content":[{"type":"thinking","thinking":"","signature":"s"},{"type":"text","text":"Nhà vua đã ngã xuống."}],
		"usage":{"input_tokens":1000,"output_tokens":1000}}`, &got, &key)

	tr := NewTranslator("Hollow Keep", NewAnthropicSend("test-key"))
	res, err := tr.Translate(context.Background(), "The king has fallen.")
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "Nhà vua đã ngã xuống." || res.USD < 0.00059 || res.USD > 0.00061 {
		t.Fatalf("result %+v", res)
	}
	if key != "test-key" || got["model"] != "claude-haiku-5-5" {
		t.Fatalf("key %q, model %v", key, got["model"])
	}
	if cfg, _ := got["output_config"].(map[string]any); cfg["effort"] != "low" {
		t.Fatalf("output_config %v", got["output_config"])
	}
	body, _ := json.Marshal(got)
	if !strings.Contains(string(body), "Hollow Keep") || !strings.Contains(string(body), "The king has fallen.") {
		t.Fatalf("prompt missing from %s", body)
	}
}

func TestAnthropicSendMapsARealAuthError(t *testing.T) {
	var got map[string]any
	var key string
	wireServer(t, 401, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`, &got, &key)
	_, err := NewAnthropicSend("secret-value")(context.Background(), "hi")
	if err == nil {
		t.Fatal("no error")
	}
	if msg := DescribeTranslateError(err); !strings.Contains(msg, "API key sai") {
		t.Fatalf("got %q", msg)
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatal("the key leaked into the error text")
	}
}

// OCR frames through to a shown translation, without the Windows parts.
func TestTypewriterLineIsTranslatedOnceEndToEnd(t *testing.T) {
	var calls []string
	tr := NewTranslator("", fakeSend(&calls, okReply))
	var s Stabilizer
	start := time.Unix(1000, 0)
	at := 0
	feed := func(text string) {
		line, ev := s.Feed(text, start.Add(time.Duration(at)*time.Millisecond))
		at += 250
		if ev == StableText {
			if _, err := tr.Translate(context.Background(), line); err != nil {
				t.Fatal(err)
			}
		}
	}
	for n := 10; n < len(stabLine); n += 8 {
		feed(stabLine[:n])
	}
	for i := 0; i < 6; i++ {
		feed(stabLine)
	}
	for i := 0; i < 5; i++ {
		feed("") // the box closes
	}
	for i := 0; i < 4; i++ {
		feed(stabLine) // the same line again: cached, no second call
	}
	if len(calls) != 1 || !strings.Contains(calls[0], stabLine) {
		t.Fatalf("%d calls: %q", len(calls), calls)
	}
}
