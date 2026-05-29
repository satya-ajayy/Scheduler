package health

import (
	// Go Internal Packages
	"context"

	// External Packages
	"go.uber.org/zap"
)

type MongoClient interface {
	Ping(ctx context.Context) error
}

type HealthCheckService struct {
	logger      *zap.Logger
	mongoClient MongoClient
}

// NewService creates a new HealthCheckService instance and returns the instance.
func NewService(logger *zap.Logger, mongoClient MongoClient) *HealthCheckService {
	return &HealthCheckService{
		logger:      logger,
		mongoClient: mongoClient,
	}
}

// Health checks the health of the database connections and returns true if all the connections are healthy.
func (h *HealthCheckService) Health(ctx context.Context) bool {
	if err := h.mongoClient.Ping(ctx); err != nil {
		h.logger.Error("Mongo Ping Failed", zap.Error(err))
		return false
	}
	return true
}
