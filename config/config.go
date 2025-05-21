package config

import (
	// Local Packages
	errors "scheduler/errors"
)

var DefaultConfig = []byte(`
application: "scheduler"

logger:
  level: "debug"

listen: ":4202"

prefix: "/scheduler"

is_prod_mode: false

mongo:
  uri: "mongodb://localhost:27017"

slack:
 webhook_url: "https://hooks.slack.com/services/your/webhook/url"
 send_alerts_in_dev: false
`)

type Config struct {
	Application string `koanf:"application"`
	Logger      Logger `koanf:"logger"`
	Listen      string `koanf:"listen"`
	Prefix      string `koanf:"prefix"`
	IsProdMode  bool   `koanf:"is_prod_mode"`
	Mongo       Mongo  `koanf:"mongo"`
	Slack       Slack  `koanf:"slack"`
}

type Logger struct {
	Level string `koanf:"level"`
}

type Mongo struct {
	URI string `koanf:"uri"`
}

type Slack struct {
	WebhookURL     string `koanf:"webhook_url"`
	SendAlertInDev bool   `koanf:"send_alerts_in_dev"`
}

// Validate validates the configuration
func (c *Config) Validate() error {
	ve := errors.ValidationErrs()

	if c.Application == "" {
		ve.Add("application", "cannot be empty")
	}
	if c.Listen == "" {
		ve.Add("listen", "cannot be empty")
	}
	if c.Logger.Level == "" {
		ve.Add("logger.level", "cannot be empty")
	}
	if c.Prefix == "" {
		ve.Add("prefix", "cannot be empty")
	}
	if c.Mongo.URI == "" {
		ve.Add("mongo.uri", "cannot be empty")
	}
	if c.Slack.WebhookURL == "" {
		ve.Add("slack.webhook_url", "cannot be empty")
	}

	return ve.Err()
}
