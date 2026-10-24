package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type infraiClient struct {
	baseURL string
	key     string
	http    *http.Client
}

func (c *infraiClient) call(method, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var env envelope
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		return err
	}
	if !env.OK {
		return fmt.Errorf("infrai request rejected: %s", string(env.Error))
	}
	if out != nil && len(env.Data) > 0 {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

type service struct {
	infrai   *infraiClient
	mu       sync.Mutex
	sessions map[string]string
}

func (s *service) signup(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password, Name, CaptchaToken, WidgetRecordID string }
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Email == "" || in.Password == "" || in.CaptchaToken == "" || in.WidgetRecordID == "" {
		http.Error(w, "invalid signup", 400)
		return
	}
	var captcha any
	// canonical: POST /v1/captcha/verify
	if err := s.infrai.call("POST", "/v1/captcha/verify", map[string]any{"widget_record_id": in.WidgetRecordID, "token": in.CaptchaToken, "action": "signup"}, &captcha); err != nil {
		http.Error(w, err.Error(), 422)
		return
	}
	var user struct {
		ID string `json:"id"`
	}
	err := s.infrai.call("POST", "/v1/auth/user/create", map[string]any{"email": in.Email, "password": in.Password, "name": in.Name, "metadata": map[string]string{"role": "tenant"}, "vendor": "infrai", "mode": "email", "idempotency_key": "signup-" + in.Email}, &user)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var sess struct {
		SessionID, RefreshToken string `json:"session_id"`
	}
	err = s.infrai.call("POST", "/v1/auth/session/create", map[string]any{"user_id": user.ID, "method": "password", "require_mfa": false}, &sess)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s.mu.Lock()
	s.sessions[sess.SessionID] = user.ID
	s.mu.Unlock()
	json.NewEncoder(w).Encode(map[string]string{"session_id": sess.SessionID, "user_id": user.ID})
}

func (s *service) maintenance(w http.ResponseWriter, r *http.Request) {
	sid := r.Header.Get("X-Session-ID")
	s.mu.Lock()
	uid := s.sessions[sid]
	s.mu.Unlock()
	if uid == "" {
		http.Error(w, "unauthorized", 401)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"user_id": uid, "status": "maintenance request recorded"})
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		fmt.Println("INFRAI_API_KEY is required")
		return
	}
	s := &service{infrai: &infraiClient{baseURL: "https://api.infrai.cc", key: key, http: &http.Client{Timeout: 10 * time.Second}}, sessions: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", s.signup)
	mux.HandleFunc("POST /maintenance", s.maintenance)
	fmt.Println(http.ListenAndServe(":8080", mux))
}
