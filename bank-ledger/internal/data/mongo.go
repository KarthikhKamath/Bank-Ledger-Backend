package data

import (
	"bank-ledger/internal/conf"
	"context"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewMongoDBConnection(c *conf.Data, logger log.Logger) (*mongo.Database, func(), error) {
	helper := log.NewHelper(logger)
	mongoURI := c.Mongodb.Uri
	dbName := c.Mongodb.Database
	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	database := client.Database(dbName)

	cleanup := func() {
		if err := client.Disconnect(context.Background()); err != nil {
			helper.Infof("Failed to disconnect MongoDB: %v", err)
		} else {
			helper.Infof("MongoDB connection closed")
		}
	}

	helper.Infof("Connected to MongoDB")

	return database, cleanup, nil
}
