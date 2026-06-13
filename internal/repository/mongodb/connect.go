package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Client wraps the mongo driver client and exposes only the operations used by this application.
type Client struct {
	driver *mongo.Client
}

// Connect connects to MongoDB and returns a wrapped Client.
func Connect(ctx context.Context, uri string) (*Client, error) {
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

	// Return the connected client.
	return &Client{driver: client}, nil
}

// Ping verifies that the MongoDB server is reachable.
func (c *Client) Ping(ctx context.Context) error {
	return c.driver.Ping(ctx, nil)
}

// Close disconnects from MongoDB.
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.driver.Disconnect(ctx)
}

// Database returns a handle to the named database.
func (c *Client) Database(name string) *mongo.Database {
	return c.driver.Database(name)
}
