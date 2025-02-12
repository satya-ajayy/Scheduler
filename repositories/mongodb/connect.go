package mongodb

import (
	// Go Internal Packages
	"context"
	"time"

	// External Packages
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Connect connects to the mongodb server and returns the client.
func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	// Set the server selection timeout to 5 seconds.
	timeout := time.Second * 5
	opts := &options.ClientOptions{ServerSelectionTimeout: &timeout}

	// Create a new MongoDB client with the provided URI and options.
	client, err := mongo.Connect(ctx, opts.ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	// Ping the MongoDB server to verify the connection.
	pingErr := client.Ping(ctx, nil)
	if pingErr != nil {
		return nil, pingErr
	}

	// Return the connected client.
	return client, nil
}
