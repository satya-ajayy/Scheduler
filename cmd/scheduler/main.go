package main

import (
	// Go Internal Packages
	"context"
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
	slack "scheduler/utils/slack"

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

// InitializeServer sets up an HTTP server with defined handlers. Repositories are initialized,
// create the services, and subsequently construct handlers for the services.
func InitializeServer(ctx context.Context, k config.Config, logger *zap.Logger) (*http.Server, error) {
	// Connect to MongoDB
	mongoClient, err := mongodb.Connect(ctx, k.Mongo.URI)
	if err != nil {
		return nil, err
	}

	// Slack Alert Sender
	slackAlerter := slack.NewSender(k.Slack, k.IsProdMode)

	// Init repos, services && handlers
	schedulerRepo := mongodb.NewSchedulerRepository(mongoClient)
	healthSvc := health.NewService(logger, mongoClient)
	schedulerSvc := scheduler.NewSchedulerService(logger, schedulerRepo, slackAlerter)

	if err = schedulerSvc.Start(ctx); err != nil {
		return nil, err
	}

	schedulerHandler := handlers.NewSchedulerHandler(schedulerSvc)
	closeCallback := func() {
		_ = mongoClient.Close()
		logger.Info("Server Stopped Successfully!")
	}

	server := http.NewServer(logger, k.Prefix, healthSvc, schedulerHandler, closeCallback)
	return server, nil
}

// LoadConfig loads the default configuration and overrides it with the config file
// specified by the path defined in the config flag.
func LoadConfig() *koanf.Koanf {
	configPath := kingpin.Flag("config", "Path To The Application Config File").
		Short('c').Default("config.yml").String()

	kingpin.Parse()

	k := koanf.New(".")
	if err := k.Load(rawbytes.Provider(config.DefaultConfig), yaml.Parser()); err != nil {
		log.Fatalf("Failed to load default config: %v", err)
	}
	if *configPath != "" {
		if err := k.Load(file.Provider(*configPath), yaml.Parser()); err != nil {
			log.Fatalf("Failed to load config file %s: %v", *configPath, err)
		}
	}
	return k
}

// NewLogger builds a production zap logger configured with logfmt encoding
// and the application's hostname and service name as initial fields.
func NewLogger(cfg config.Config) *zap.Logger {
	zapCfg := zap.NewProductionConfig()
	zapCfg.Encoding = cfg.Logger.Encoding
	_ = zapCfg.Level.UnmarshalText([]byte(cfg.Logger.Level))
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapCfg.OutputPaths = []string{"stdout"}

	hostname, _ := os.Hostname()
	zapCfg.InitialFields = map[string]any{
		"host":    hostname,
		"service": cfg.Application,
	}

	logger, err := zapCfg.Build()
	if err != nil {
		log.Fatalf("Failed To Initialize Logger: %v", err)
	}
	return logger
}

// main is the entrypoint that loads config, sets up logging,
// and starts the HTTP server with graceful shutdown.
func main() {
	k := LoadConfig()

	// Unmarshal config into struct
	appKonf := config.Config{}
	if err := k.Unmarshal("", &appKonf); err != nil {
		log.Fatalf("Error Loading Config: %v", err)
	}

	// Print the config
	if !appKonf.IsProdMode {
		helpers.PrintStruct(appKonf)
	}

	// Validate the config
	if err := appKonf.Validate(); err != nil {
		helpers.LogValidationErrors(err)
		log.Fatalf("Invalid Configuration!")
	}

	logger := NewLogger(appKonf)
	defer func() {
		_ = logger.Sync()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv, err := InitializeServer(ctx, appKonf, logger)
	if err != nil {
		logger.Fatal("Cannot Initialize Server!", zap.Error(err))
	}

	if err = srv.Listen(ctx, appKonf.Listen); err != nil {
		logger.Fatal("Cannot Listen On Port!", zap.Error(err))
	}
}
