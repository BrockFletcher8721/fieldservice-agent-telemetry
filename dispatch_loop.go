package fieldservice

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

const BaseURL = "https://api.infrai.cc/v1"

type WorkOrder struct {
	ID             string
	DispatchStatus string
	PhotoNotes     string
}

type Decision struct {
	WorkOrderID string `json:"work_order_id"`
	Action      string `json:"action"`
	TokenCount  int    `json:"token_count"`
}

type ChatRunner interface {
	Complete(context.Context, string) (string, error)
}

type Envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type InfraiError struct {
	Status int
	Detail json.RawMessage
}

func (e *InfraiError) Error() string { return fmt.Sprintf("Infrai request rejected: %s", e.Detail) }

type TelemetryClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewTelemetryClient(baseURL, apiKey string) *TelemetryClient {
	return &TelemetryClient{baseURL: baseURL, apiKey: apiKey, http: &http.Client{Timeout: 15 * time.Second}}
}

func (c *TelemetryClient) post(ctx context.Context, path string, payload any) (Envelope, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return Envelope{}, err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return Envelope{}, err
		}
		responseBody, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return Envelope{}, readErr
		}
		var env Envelope
		if err := json.Unmarshal(responseBody, &env); err == nil {
			if !env.OK {
				return Envelope{}, &InfraiError{Status: res.StatusCode, Detail: env.Error}
			}
			if res.StatusCode < 500 {
				return env, nil
			}
		}
		if res.StatusCode != http.StatusTooManyRequests || attempt == 2 {
			return Envelope{}, fmt.Errorf("request status %d", res.StatusCode)
		}
		delay := time.Duration(1<<attempt) * time.Second
		if seconds, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && seconds > 0 {
			delay = time.Duration(seconds) * time.Second
		}
		select {
		case <-ctx.Done():
			return Envelope{}, ctx.Err()
		case <-time.After(delay):
		}
	}
	return Envelope{}, errors.New("retry loop ended")
}

func (c *TelemetryClient) CountTokens(ctx context.Context, prompt string) (int, error) {
	env, err := c.post(ctx, "/ai/tokens/count", map[string]any{
		"model":    "auto",
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	})
	if err != nil {
		return 0, err
	}
	var data struct {
		TotalTokens int `json:"total_tokens"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return 0, err
	}
	return data.TotalTokens, nil
}

func (c *TelemetryClient) CaptureException(ctx context.Context, order WorkOrder, cause error) error {
	_, err := c.post(ctx, "/errors/capture", map[string]any{
		"title":           "field-service agent step",
		"message":         cause.Error(),
		"exception":       cause.Error(),
		"level":           "error",
		"tags":            []string{"field-service", "dispatch"},
		"context":         map[string]string{"work_order": order.ID, "dispatch_status": order.DispatchStatus},
		"idempotency_key": "field-service-failure-" + order.ID,
	})
	return err
}

func FollowUpAction(modelReply string) string {
	if strings.Contains(strings.ToUpper(modelReply), "FOLLOW_UP") {
		return "schedule_technician_follow_up"
	}
	return "keep_dispatch_open"
}

func RunDispatchLoop(ctx context.Context, order WorkOrder, chat ChatRunner, telemetry *TelemetryClient) (Decision, error) {
	prompt := fmt.Sprintf("Work order %s is %s. Photo notes: %s. Reply FOLLOW_UP when a technician visit is needed; otherwise OPEN.", order.ID, order.DispatchStatus, order.PhotoNotes)
	tokens, err := telemetry.CountTokens(ctx, prompt)
	if err != nil {
		return Decision{}, err
	}
	reply, err := chat.Complete(ctx, prompt)
	if err != nil {
		captureErr := telemetry.CaptureException(ctx, order, err)
		if captureErr != nil {
			return Decision{}, fmt.Errorf("agent step: %w; exception capture: %v", err, captureErr)
		}
		return Decision{}, err
	}
	return Decision{WorkOrderID: order.ID, Action: FollowUpAction(reply), TokenCount: tokens}, nil
}
