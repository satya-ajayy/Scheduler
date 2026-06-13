package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"scheduler/internal/config"
	"scheduler/internal/logger"
	"scheduler/internal/repository/mongodb"
	"scheduler/internal/service/health"
	"scheduler/internal/service/scheduler"
	"scheduler/internal/transport"
	"scheduler/internal/transport/handler"
	"scheduler/pkg/httpclient"
	"scheduler/pkg/notifier"

	"go.uber.org/zap"
)

func main() {
	// Load and validate configuration from file and defaults
	cfg := config.Load()

	// Initialize structured logger
	log := logger.New(cfg)
	defer func() { _ = log.Sync() }()

	// Listen for OS shutdown signals for graceful termination
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Connect to MongoDB
	mongoClient, err := mongodb.Connect(ctx, cfg.Mongo.URI)
	if err != nil {
		log.Fatal("Cannot Connect To MongoDB!", zap.Error(err))
	}

	// Initialize Slack alert sender
	slackAlerter := notifier.NewSlackSender(cfg.Slack, cfg.IsProdMode)

	// Initialize shared HTTP client with connection pool
	httpClient := httpclient.New()

	// Wire repositories, services and handlers
	schedulerRepo := mongodb.NewSchedulerRepository(mongoClient)
	healthSvc := health.NewService(mongoClient)
	schedulerSvc := scheduler.NewService(ctx, log, schedulerRepo, slackAlerter, httpClient)

	// Load active tasks from DB and start the cron runner
	if err = schedulerSvc.Start(ctx); err != nil {
		log.Fatal("Cannot Start Scheduler!", zap.Error(err))
	}

	schedulerHandler := handler.NewSchedulerHandler(schedulerSvc)

	// Close MongoDB connection on shutdown
	closeCallback := func() {
		_ = mongoClient.Close()
		log.Info("Server Stopped Successfully!")
	}

	// Start HTTP server and block until shutdown signal
	srv := transport.NewServer(log, cfg.Prefix, healthSvc, schedulerHandler, closeCallback)
	if err = srv.Listen(ctx, cfg.Listen); err != nil {
		log.Fatal("Cannot Listen On Port!", zap.Error(err))
	}
}
