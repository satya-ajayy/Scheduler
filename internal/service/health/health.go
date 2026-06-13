package health

import (
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
		return fmt.Errorf("mongo ping: %w", err)
	}
	return nil
}
