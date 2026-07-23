package telegramclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client interface {
	SendMessage(ctx context.Context, token, chatID, text, parseMode string) error
	SendMessageWithMarkup(ctx context.Context, token, chatID, text, parseMode, replyMarkup string) error
	GetUpdates(ctx context.Context, token string, offset int64, timeout int) ([]Update, error)
	AnswerCallbackQuery(ctx context.Context, token, callbackQueryID string) error
}

type defaultClient struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient() Client {
	return &defaultClient{
		httpClient: &http.Client{Timeout: 65 * time.Second}, // Slightly longer than long-polling timeout
		baseURL:    "https://api.telegram.org",
	}
}

type tgResponse struct {
	Ok          bool            `json:"ok"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Description string          `json:"description,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

type Message struct {
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	Text string `json:"text"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	Data    string   `json:"data"`
	Message *Message `json:"message"`
}

func (c *defaultClient) do(ctx context.Context, token, method string, params url.Values, target interface{}) error {
	u := fmt.Sprintf("%s/bot%s/%s", c.baseURL, token, method)

	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Do not return raw network errors as they may contain the token in the URL
		return fmt.Errorf("network error occurred while connecting to Telegram API")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var tr tgResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		if resp.StatusCode != 200 {
			return fmt.Errorf("http error %d: unable to parse response", resp.StatusCode)
		}
		return fmt.Errorf("failed to parse response JSON: %w", err)
	}

	if !tr.Ok {
		return fmt.Errorf("telegram API error (code %d): %s", tr.ErrorCode, tr.Description)
	}

	if target != nil {
		if err := json.Unmarshal(tr.Result, target); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return nil
}

func (c *defaultClient) SendMessage(ctx context.Context, token, chatID, text, parseMode string) error {
	return c.SendMessageWithMarkup(ctx, token, chatID, text, parseMode, "")
}

func (c *defaultClient) SendMessageWithMarkup(ctx context.Context, token, chatID, text, parseMode, replyMarkup string) error {
	params := url.Values{
		"chat_id": {chatID},
		"text":    {text},
	}
	if parseMode != "" {
		params.Set("parse_mode", parseMode)
	}
	params.Set("disable_web_page_preview", "true")
	if replyMarkup != "" {
		params.Set("reply_markup", replyMarkup)
	}

	return c.do(ctx, token, "sendMessage", params, nil)
}

func (c *defaultClient) GetUpdates(ctx context.Context, token string, offset int64, timeout int) ([]Update, error) {
	params := url.Values{
		"offset":  {fmt.Sprint(offset)},
		"timeout": {fmt.Sprint(timeout)},
	}

	var updates []Update
	err := c.do(ctx, token, "getUpdates", params, &updates)
	return updates, err
}

func (c *defaultClient) AnswerCallbackQuery(ctx context.Context, token, callbackQueryID string) error {
	params := url.Values{
		"callback_query_id": {callbackQueryID},
	}
	return c.do(ctx, token, "answerCallbackQuery", params, nil)
}
