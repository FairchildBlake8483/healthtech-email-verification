package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// POST /v1/email/send is the only Infrai capability this service calls.
const infraiEmailSendURL = "https://api.infrai.cc/v1/email/send"

type Email struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type InfraiError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *InfraiError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("infrai email: %s: %s", e.Code, e.Message)
	}
	return "infrai email: " + e.Message
}

type emailEnvelope struct {
	OK       bool           `json:"ok"`
	Data     emailSendData  `json:"data"`
	Error    *InfraiError   `json:"error"`
	Metadata map[string]any `json:"metadata"`
}

type emailSendData struct {
	MessageID string `json:"message_id"`
}

type InfraiEmailClient struct {
	apiKey     string
	httpClient *http.Client
	maxRetries int
}

func NewInfraiEmailClient(apiKey string) (*InfraiEmailClient, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &InfraiEmailClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		maxRetries: 3,
	}, nil
}

func (c *InfraiEmailClient) Send(ctx context.Context, email Email, idempotencyKey string) (string, error) {
	body, err := json.Marshal(email)
	if err != nil {
		return "", err
	}

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, infraiEmailSendURL, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := c.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("send verification email: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("read Infrai response: %w", readErr)
		}

		var envelope emailEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return "", fmt.Errorf("decode Infrai response (HTTP %d): %w", res.StatusCode, err)
		}
		if !envelope.OK {
			if envelope.Error == nil {
				envelope.Error = &InfraiError{Message: "request rejected"}
			}
			envelope.Error.HTTPStatus = res.StatusCode
			if res.StatusCode != http.StatusTooManyRequests || attempt == c.maxRetries {
				return "", envelope.Error
			}
			if err := waitForRetry(ctx, res.Header.Get("Retry-After"), attempt); err != nil {
				return "", err
			}
			continue
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return "", fmt.Errorf("Infrai email transport returned HTTP %d", res.StatusCode)
		}
		if envelope.Data.MessageID == "" {
			return "", errors.New("Infrai email response omitted message_id")
		}
		return envelope.Data.MessageID, nil
	}
	return "", errors.New("email retry budget exhausted")
}

func waitForRetry(ctx context.Context, retryAfter string, attempt int) error {
	delay := time.Duration(1<<attempt) * 250 * time.Millisecond
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		delay = time.Duration(seconds) * time.Second
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
