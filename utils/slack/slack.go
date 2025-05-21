package slack

import (
	// Go Internal Packages
	"bytes"
	"encoding/json"
	"fmt"
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
func NewSender(config config.Slack, isProd bool) Sender {
	return &SlackSender{
		client: &http.Client{Timeout: 5 * time.Second},
		config: config, isProd: isProd,
	}
}

func (s *SlackSender) SendAlert(task models.TaskModel, errText string) error {
	if s.isProd || (!s.isProd && s.config.SendAlertInDev) {
		header := Block{
			Type: "header",
			Text: Text{
				Type: "plain_text",
				Text: fmt.Sprintf("Exception In Scheduler Service"),
			},
		}

		body := Block{
			Type: "section",
			Text: Text{
				Type: "mrkdwn",
				Text: fmt.Sprintf("```TaskID: %s\nError: %s\n```", task.ID, errText),
			},
		}

		payload := Payload{
			Blocks: []Block{header, body},
		}

		jsonPayload, _ := json.Marshal(payload)
		_, err := s.client.Post(s.config.WebhookURL, "application/json", bytes.NewReader(jsonPayload))
		return err
	}
	return nil
}
