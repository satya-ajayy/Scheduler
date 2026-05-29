package slack

import (
	// Go Internal Packages
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	// Local Packages
	config "scheduler/config"
	models "scheduler/models"
)

type Text struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type Block struct {
	Type string `json:"type"`
	Text Text   `json:"text"`
}

type Payload struct {
	Blocks []Block `json:"blocks"`
}

type SlackSender struct {
	client *http.Client
	config config.Slack
	isProd bool
}

type Sender interface {
	SendAlert(task models.TaskModel, errText string) error
}

// NewSender creates a new Slack alert sender
func NewSender(cfg config.Slack, isProd bool) Sender {
	return &SlackSender{
		client: &http.Client{Timeout: 5 * time.Second},
		config: cfg,
		isProd: isProd,
	}
}

func (s *SlackSender) SendAlert(task models.TaskModel, errText string) error {
	if !s.isProd && !s.config.SendAlertInDev {
		return nil
	}

	header := Block{
		Type: "header",
		Text: Text{
			Type: "plain_text",
			Text: "Exception In Scheduler Service",
		},
	}
	body := Block{
		Type: "section",
		Text: Text{
			Type: "mrkdwn",
			Text: fmt.Sprintf("```TaskID: %s\nError: %s\n```", task.ID, errText),
		},
	}
	payload := Payload{Blocks: []Block{header, body}}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	resp, err := s.client.Post(s.config.WebhookURL, "application/json", bytes.NewReader(jsonPayload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
