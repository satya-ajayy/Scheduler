package main

import (
	// Go Internal Packages
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	// Local Packages
	config "scheduler/config"
	http "scheduler/http"
	handlers "scheduler/http/handlers"
	mongodb "scheduler/repositories/mongodb"
	health "scheduler/services/health"
	scheduler "scheduler/services/scheduler"
	helpers "scheduler/utils/helpers"
	httpclient "scheduler/utils/httpclient"
	notifications "scheduler/utils/notifications"

	// External Packages
	"github.com/alecthomas/kingpin/v2"
	_ "github.com/jsternberg/zap-logfmt"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitializeServer sets up the HTTP server with all dependencies wired together:
// MongoDB → Repositories → Services → Handlers → Server
func InitializeServer(ctx context.Context, k config.Config, logger *zap.Logger) (*http.Server, error) {
	// MongoDB
	mongoClient, err := mongodb.Connect(ctx, logger, k.Mongo.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect mongo: %w", err)
	}

	// Initialize Slack alert sender
	slackAlerter := notifications.NewSlackSender(k.Slack, k.IsProdMode)

	// Initialize shared HTTP client with connection pool
	httpClient := httpclient.New()

	// Wire repositories, services and handlers
	schedulerRepo := mongodb.NewSchedulerRepository(mongoClient)
	healthSVC := health.NewService(mongoClient)
	schedulerSVC := scheduler.NewService(logger, schedulerRepo, slackAlerter, httpClient)

	closeCallback := func() {
		schedulerSVC.Stop()
		_ = mongoClient.Close()
		logger.Info("Server Stopped Successfully")
	}

	// Load active tasks from DB and start the cron runner
	if err = schedulerSVC.Start(ctx); err != nil {
		logger.Fatal("Cannot Start Scheduler!", zap.Error(err))
	}

	schedulerHandler := handlers.NewSchedulerHandler(schedulerSVC)
	server := http.NewServer(logger, k.Prefix, healthSVC, schedulerHandler, closeCallback)
	return server, nil

}

// LoadConfig loads the default configuration and overrides it with the config file
// specified by the --config flag.
func LoadConfig() *koanf.Koanf {
	configPath := kingpin.Flag("config", "Path To The Application Config File").
		Short('c').Default("config.yml").String()

	kingpin.Parse()

	k := koanf.New(".")
	_ = k.Load(rawbytes.Provider(config.DefaultConfig), yaml.Parser())
	if *configPath != "" {
		_ = k.Load(file.Provider(*configPath), yaml.Parser())
	}
	return k
}

// NewLogger builds a production zap logger configured with logfmt encoding
// and the application's hostname and service name as initial fields.
func NewLogger(k config.Config) *zap.Logger {
	zapCfg := zap.NewProductionConfig()
	zapCfg.Encoding = k.Logger.Encoding
	_ = zapCfg.Level.UnmarshalText([]byte(k.Logger.Level))
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapCfg.OutputPaths = []string{"stdout"}

	hostname, _ := os.Hostname()
	zapCfg.InitialFields = map[string]any{
		"host":    hostname,
		"service": k.Application,
	}

	logger, _ := zapCfg.Build()
	return logger
}

// main is the entrypoint that loads config, sets up logging,
// and starts the HTTP server with graceful shutdown.
func main() {
	k := LoadConfig()

	// Unmarshal Config
	appKonf := config.Config{}
	if err := k.Unmarshal("", &appKonf); err != nil {
		log.Fatalf("Error Loading Config: %v", err)
	}

	// Validate Config
	if err := appKonf.Validate(); err != nil {
		helpers.LogValidationErrors(err)
		log.Fatalf("Invalid Configuration")
	}

	// Print Config in Dev Mode
	if !appKonf.IsProdMode {
		helpers.PrintStruct(appKonf)
	}

	// Initialize Logger
	logger := NewLogger(appKonf)
	defer func() {
		_ = logger.Sync()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv, err := InitializeServer(ctx, appKonf, logger)
	if err != nil {
		logger.Fatal("Cannot Initialize Server", zap.Error(err))
	}

	if err = srv.Listen(ctx, appKonf.Listen); err != nil {
		logger.Fatal("Cannot Listen On Port", zap.Error(err))
	}
}
