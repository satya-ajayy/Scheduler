package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	config "scheduler/internal/config"
	task "scheduler/internal/task"
)

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackBlock struct {
	Type string    `json:"type"`
	Text slackText `json:"text"`
}

type slackPayload struct {
	Blocks []slackBlock `json:"blocks"`
}

type slackSender struct {
	client *http.Client
	config config.Slack
	isProd bool
}

func NewSlackSender(cfg config.Slack, isProd bool) Sender {
	return &slackSender{
		client: &http.Client{Timeout: 5 * time.Second},
		config: cfg,
		isProd: isProd,
	}
}

func (s *slackSender) SendAlert(ctx context.Context, t task.Task, errMsg string) error {
	if !s.isProd && !s.config.SendAlertInDev {
		return nil
	}

	payload := slackPayload{
		Blocks: []slackBlock{
			{
				Type: "header",
				Text: slackText{Type: "plain_text", Text: "Exception In Scheduler Service"},
			},
			{
				Type: "section",
				Text: slackText{Type: "mrkdwn", Text: fmt.Sprintf("```TaskID: %s\nError: %s\n```", t.ID, errMsg)},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack alert: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack returned non-2xx status: %d", resp.StatusCode)
	}
	return nil
}
