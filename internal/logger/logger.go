package logger

import (
	"log"
	"os"

	config "scheduler/internal/config"

	_ "github.com/jsternberg/zap-logfmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New builds a production zap logger configured with logfmt encoding
// and the application's hostname and service name as initial fields.
func New(cfg config.Config) *zap.Logger {
	zapCfg := zap.NewProductionConfig()
	zapCfg.Encoding = cfg.Logger.Encoding
	if err := zapCfg.Level.UnmarshalText([]byte(cfg.Logger.Level)); err != nil {
		log.Fatalf("Failed To Initialize Logger: invalid log level %q: %v", cfg.Logger.Level, err)
	}
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapCfg.OutputPaths = []string{"stdout"}

	hostname, _ := os.Hostname()
	zapCfg.InitialFields = map[string]any{
		"host":    hostname,
		"service": cfg.Application,
	}

	l, err := zapCfg.Build()
	if err != nil {
		log.Fatalf("Failed To Initialize Logger: %v", err)
	}
	return l
}
