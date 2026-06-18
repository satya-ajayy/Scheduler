package mongodb

import (
	// Go Internal Packages
	"context"
	"time"

	// External Packages
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
)

type Client struct {
	client *mongo.Client
}

// Connect connects to the mongodb server and returns the client.
func Connect(ctx context.Context, logger *zap.Logger, uri string) (*Client, error) {
	// Set the server selection timeout to 5 seconds.
	timeout := time.Second * 5
	opts := options.Client().ApplyURI(uri).SetServerSelectionTimeout(timeout)

	// Create a new MongoDB client with the provided URI and options.
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}

	// Ping the MongoDB server to verify the connection.
	if err = client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	logger.Info("Connected To MongoDB Successfully")
	return &Client{client: client}, nil
}

func (c *Client) Database(name string) *mongo.Database {
	return c.client.Database(name)
}

func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx, nil)
}

func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.client.Disconnect(ctx)
}
