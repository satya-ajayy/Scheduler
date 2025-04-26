package utils

import (
	// Go Internal Packages
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	// Local Packages
	config "scheduler/config"
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

type Sender = func(title string, err error) error

func NewSender(k config.Slack, isProd bool) Sender {
	return func(title string, err error) error {
		if isProd || (!isProd && k.SendAlertInDev) {
			header := Block{
				Type: "header",
				Text: Text{
					Type: "plain_text",
					Text: fmt.Sprintf("Scheduler: %s", title),
				},
			}

			var errorText string
			if err != nil {
				errorText = err.Error()
			} else {
				errorText = "No specific error details available"
			}

			body := Block{
				Type: "section",
				Text: Text{
					Type: "mrkdwn",
					Text: fmt.Sprintf("```\n%s\n```", errorText),
				},
			}
			payload := Payload{
				Blocks: []Block{header, body},
			}
			jsonPayload, _ := json.Marshal(payload)
			_, postErr := http.Post(k.WebhookURL, "application/json", bytes.NewReader(jsonPayload))
			return postErr
		}
		return nil
	}
}
