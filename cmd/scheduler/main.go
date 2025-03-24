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
	shttp "scheduler/http"
	handlers "scheduler/http/handlers"
	mongodb "scheduler/repositories/mongodb"
	health "scheduler/services/health"
	scheduler "scheduler/services/scheduler"

	// External Packages
	"github.com/alecthomas/kingpin/v2"
	_ "github.com/jsternberg/zap-logfmt"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"go.uber.org/zap"
)

// InitializeServer sets up an HTTP server with defined handlers. Repositories are initialized,
// creates the services, and subsequently constructs handlers for the services
func InitializeServer(ctx context.Context, k config.Config, logger *zap.Logger) (*shttp.Server, error) {
	// Connect to mongodb
	mongoClient, err := mongodb.Connect(ctx, k.Mongo.URI)
	if err != nil {
		return nil, err
	}

	// Init repos, services && handlers
	schedulerRepo := mongodb.NewSchedulerRepository(mongoClient)
	schedulerSvc := scheduler.NewSchedulerService(schedulerRepo, logger)
	healthSvc := health.NewService(logger, mongoClient)
	err = schedulerSvc.Start(ctx)
	if err != nil {
		return nil, err
	}
	schedulerHandler := handlers.NewSchedulerHandler(schedulerSvc)
	server := shttp.NewServer(k.Prefix, logger, healthSvc, schedulerHandler)
	return server, nil
}

// LoadConfig loads the default configuration and overrides it with the config file
// specified by the path defined in the config flag
func LoadConfig() *koanf.Koanf {
	configPathMsg := "Path to the application config file"
	configPath := kingpin.Flag("config", configPathMsg).Short('c').Default("config.yml").String()

	kingpin.Parse()
	k := koanf.New(".")
	_ = k.Load(rawbytes.Provider(config.DefaultConfig), yaml.Parser())
	if *configPath != "" {
		_ = k.Load(file.Provider(*configPath), yaml.Parser())
	}

	return k
}

func main() {
	k := LoadConfig()
	appKonf := config.Config{}

	// Unmarshalling config into struct
	err := k.Unmarshal("", &appKonf)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Validate the config loaded
	if err = appKonf.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	if !appKonf.IsProdMode {
		k.Print()
	}

	cfg := zap.NewProductionConfig()
	cfg.Encoding = "logfmt"
	_ = cfg.Level.UnmarshalText([]byte(appKonf.Logger.Level))
	cfg.InitialFields = make(map[string]any)
	cfg.InitialFields["host"], _ = os.Hostname()
	cfg.InitialFields["service"] = appKonf.Application
	cfg.OutputPaths = []string{"stdout"}
	logger, _ := cfg.Build()
	defer func() {
		_ = logger.Sync()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv, err := InitializeServer(ctx, appKonf, logger)
	if err != nil {
		logger.Fatal("Cannot initialize server", zap.Error(err))
	}
	if err := srv.Listen(ctx, appKonf.Listen); err != nil {
		logger.Fatal("Cannot listen", zap.Error(err))
	}
}
