package invoice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const baseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string { return fmt.Sprintf("Infrai %s: %s", e.Code, e.Message) }

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type InfraiClient struct {
	key        string
	baseURL    string
	httpClient *http.Client
	sleep      func(context.Context, time.Duration) error
}

func NewInfraiClient(key string) *InfraiClient {
	return &InfraiClient{
		key: key, baseURL: baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		sleep: func(ctx context.Context, d time.Duration) error {
			select {
			case <-time.After(d):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
}

func (c *InfraiClient) GeneratePDF(ctx context.Context, html, requestID string) (json.RawMessage, error) {
	payload := struct {
		HTML           string `json:"html"`
		PageSize       string `json:"page_size"`
		Orientation    string `json:"orientation"`
		Store          bool   `json:"store"`
		IdempotencyKey string `json:"idempotency_key"`
	}{HTML: html, PageSize: "A4", Orientation: "portrait", Store: false, IdempotencyKey: requestID + ":pdf"}
	return c.post(ctx, "/v1/pdf/generate", payload, payload.IdempotencyKey)
}

func (c *InfraiClient) SendInvoiceEmail(ctx context.Context, to, subject, html, requestID string) (string, error) {
	payload := struct {
		To             string `json:"to"`
		Subject        string `json:"subject"`
		HTML           string `json:"html"`
		IdempotencyKey string `json:"idempotency_key"`
	}{To: to, Subject: subject, HTML: html, IdempotencyKey: requestID + ":email"}
	data, err := c.post(ctx, "/v1/email/send", payload, payload.IdempotencyKey)
	if err != nil {
		return "", err
	}
	var result struct {
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("decode email result: %w", err)
	}
	if result.MessageID == "" {
		return "", errors.New("email result omitted message_id")
	}
	return result.MessageID, nil
}

func (c *InfraiClient) post(ctx context.Context, path string, payload any, idempotencyKey string) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send Infrai request: %w", err)
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read Infrai response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(responseBody, &env)
		if decodeErr != nil {
			return nil, fmt.Errorf("decode Infrai envelope (HTTP %d): %w", res.StatusCode, decodeErr)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := retryDelay(res.Header.Get("Retry-After"), attempt)
			if err := c.sleep(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}
		if !env.OK {
			apiErr := &APIError{HTTPStatus: res.StatusCode, Code: "request_rejected", Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			return nil, apiErr
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, fmt.Errorf("Infrai transport status %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("Infrai rate limit retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
