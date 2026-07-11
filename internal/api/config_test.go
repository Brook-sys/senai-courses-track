package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Brook-sys/senai-courses-track/internal/storage"
)

func TestGetConfigDoesNotExposeTelegramToken(t *testing.T) {
	db, err := storage.New(t.TempDir() + "/courses.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	const secret = "123456:secret-token"
	if err := db.SetConfig("telegram_token", secret); err != nil {
		t.Fatal(err)
	}
	if err := db.SetConfig("telegram_chat_id", "987654321"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	res := httptest.NewRecorder()
	NewRouter(db, nil, nil).ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if strings.Contains(res.Body.String(), secret) {
		t.Fatalf("response exposed Telegram token: %s", res.Body.String())
	}

	var body struct {
		TelegramTokenConfigured bool   `json:"telegram_token_configured"`
		TelegramChatID          string `json:"telegram_chat_id"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.TelegramTokenConfigured {
		t.Fatal("telegram_token_configured = false, want true")
	}
	if body.TelegramChatID != "987654321" {
		t.Fatalf("telegram_chat_id = %q, want %q", body.TelegramChatID, "987654321")
	}
}
