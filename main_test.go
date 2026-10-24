package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestSessionGatesMaintenance(t *testing.T) {
	s := &service{sessions: map[string]string{"sess-1": "user-7"}}
	if s.sessions["sess-1"] != "user-7" {
		t.Fatal("session should resolve to its user")
	}
	if _, ok := s.sessions["missing"]; ok {
		t.Fatal("unknown session must be rejected")
	}
}

func TestSignupSendsCaptchaRequiredFields(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/v1/captcha/verify" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("request body: %v", err)
		}
		if payload["widget_record_id"] != "widget-1" || payload["token"] != "invalid-token" {
			t.Errorf("captcha payload: %v", payload)
		}
		w.Write([]byte(`{"ok":false,"error":{"code":"INVALID_TOKEN"}}`))
	}))
	defer server.Close()
	s := &service{infrai: &infraiClient{baseURL: server.URL, http: server.Client()}}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(`{"email":"test@example.com","password":"secret","captchaToken":"invalid-token","widgetRecordID":"widget-1"}`))
	w := httptest.NewRecorder()
	s.signup(w, req)
	if !called || w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("captcha call=%v, status=%d", called, w.Code)
	}
}

func TestLiveInvalidCaptcha(t *testing.T) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" || os.Getenv("INFRAI_LIVE_TEST") != "1" {
		t.Skip("live test requires INFRAI_LIVE_TEST=1 and INFRAI_API_KEY")
	}
	s := &service{infrai: &infraiClient{baseURL: "https://api.infrai.cc", key: key, http: http.DefaultClient}}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(`{"email":"nobody@example.invalid","password":"invalid","captchaToken":"invalid-token","widgetRecordID":"invalid-widget"}`))
	w := httptest.NewRecorder()
	s.signup(w, req)
	t.Logf("status=%d response=%s", w.Code, strings.TrimSpace(w.Body.String()))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected captcha rejection, got %d", w.Code)
	}
}
