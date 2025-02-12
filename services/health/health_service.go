package health

import (
	// Go Internal Packages
	"context"

	// External Packages
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type HealthCheckerService struct {
	logger      *zap.Logger
	mongoClient *mongo.Client
}

// NewService creates a new HealthCheckerService instance and returns the instance.
func NewService(logger *zap.Logger, mongoClient *mongo.Client) *HealthCheckerService {
	return &HealthCheckerService{
		logger:      logger,
		mongoClient: mongoClient,
	}
}

// Health checks the health of the database connections and returns true if all the connections are healthy.
func (h *HealthCheckerService) Health(ctx context.Context) bool {
	// check mongo ping
	if mongoPingErr := h.mongoClient.Ping(ctx, nil); mongoPingErr != nil {
		h.logger.Error("Mongo ping failed", zap.Error(mongoPingErr))
		return false
	}
	return true
}
