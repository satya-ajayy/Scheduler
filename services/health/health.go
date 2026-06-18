package health

import (
	// Go Internal Packages
	"context"
	"fmt"
)

type MongoClient interface {
	Ping(ctx context.Context) error
}

type HealthCheckService struct {
	mongoClient MongoClient
}

func NewService(mongoClient MongoClient) *HealthCheckService {
	return &HealthCheckService{mongoClient: mongoClient}
}

func (h *HealthCheckService) Health(ctx context.Context) error {
	if err := h.mongoClient.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping mongo: %w", err)
	}
	return nil
}
