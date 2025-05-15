package data

import (
	"context"
	"fmt"
	"github.com/go-kratos/kratos/v2/errors"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"

	"bank-ledger/internal/entity"
	"github.com/go-kratos/kratos/v2/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"
)

type TransactionLogsRepository interface {
	CreateTransaction(ctx context.Context, txn *entity.TransactionLog) error
	AppendTransactionLog(ctx context.Context, txnID string, retryAttempt int, entry entity.LogEntry) error
	GetTransaction(ctx context.Context, txnID string) (*entity.TransactionLog, error)
}

type TransactionLogsRepo struct {
	data  *Data
	db    *gorm.DB
	log   *log.Helper
	mongo *mongo.Database
}

func NewTransactionLogsRepo(data *Data, logger log.Logger, mongoDB *mongo.Database) TransactionLogsRepository {
	return &TransactionLogsRepo{
		data:  data,
		db:    data.db,
		log:   log.NewHelper(logger),
		mongo: mongoDB,
	}
}

func (r *TransactionLogsRepo) CreateTransaction(ctx context.Context, txn *entity.TransactionLog) error {
	collection := r.mongo.Collection("transactions")
	_, err := collection.InsertOne(ctx, txn)
	if err != nil {
		return fmt.Errorf("failed to create transaction in MongoDB: %w", err)
	}
	r.log.Infof("Created transaction %s", txn.TransactionID)
	return nil
}

func (r *TransactionLogsRepo) AppendTransactionLog(ctx context.Context, txnID string, retryAttempt int, entry entity.LogEntry) error {
	collection := r.mongo.Collection("transactions")
	filter := bson.M{"transactionid": txnID}
	timestamp := time.Now()

	logEntry := entry
	logEntry.Timestamp = timestamp

	var txn struct {
		TransactionLogs []struct {
			Attempt int `bson:"attempt"`
		} `bson:"transactionLogs"`
	}
	err := collection.FindOne(ctx, filter, options.FindOne().SetProjection(
		bson.M{"transactionLogs.attempt": 1},
	)).Decode(&txn)

	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return fmt.Errorf("failed to query transaction: %w", err)
	}

	attemptExists := false
	if err == nil {
		for _, tryLog := range txn.TransactionLogs {
			if tryLog.Attempt == retryAttempt {
				attemptExists = true
				break
			}
		}
	}

	var updateResult *mongo.UpdateResult
	if attemptExists {
		update := bson.M{
			"$push": bson.M{
				"transactionLogs.$[elem].logs": logEntry,
			},
			"$set": bson.M{
				"status":    entry.Status,
				"message":   entry.Message,
				"timestamp": timestamp,
			},
		}

		opts := options.Update().SetArrayFilters(options.ArrayFilters{
			Filters: []interface{}{
				bson.M{"elem.attempt": retryAttempt},
			},
		})

		updateResult, err = collection.UpdateOne(ctx, filter, update, opts)
	} else {
		update := bson.M{
			"$push": bson.M{
				"transactionLogs": bson.M{
					"attempt":   retryAttempt,
					"logs":      []entity.LogEntry{logEntry},
					"timestamp": timestamp,
				},
			},
			"$set": bson.M{
				"status":    entry.Status,
				"message":   entry.Message,
				"trycount":  retryAttempt,
				"timestamp": timestamp,
			},
		}

		updateResult, err = collection.UpdateOne(ctx, filter, update)
	}

	if err != nil {
		return fmt.Errorf("failed to append transaction log: %w", err)
	}

	if updateResult.MatchedCount == 0 {
		r.log.Warnf("Transaction %s not found, creating it now", txnID)

		newTxn := bson.M{
			"transactionid": txnID,
			"timestamp":     timestamp,
			"status":        entry.Status,
			"message":       entry.Message,
			"trycount":      retryAttempt,
			"transactionLogs": []bson.M{
				{
					"attempt":   retryAttempt,
					"logs":      []entity.LogEntry{logEntry},
					"timestamp": timestamp,
				},
			},
		}

		_, err := collection.InsertOne(ctx, newTxn)
		if err != nil {
			return fmt.Errorf("failed to create new transaction log: %w", err)
		}
	}

	r.log.Infof("Appended log for transaction %s (retry #%d)", txnID, retryAttempt)
	return nil
}

func (r *TransactionLogsRepo) GetTransaction(ctx context.Context, txnID string) (*entity.TransactionLog, error) {
	collection := r.mongo.Collection("transactions")
	filter := bson.M{"transactionid": txnID}

	var txn entity.TransactionLog
	err := collection.FindOne(ctx, filter).Decode(&txn)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("transaction not found: %w", err)
		}
		return nil, fmt.Errorf("failed to retrieve transaction: %w", err)
	}

	return &txn, nil
}
