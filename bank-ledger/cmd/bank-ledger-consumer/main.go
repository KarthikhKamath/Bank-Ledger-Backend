package main

import (
	v1 "bank-ledger/api/bankLedger/v1"
	"bank-ledger/internal/conf"
	"bank-ledger/internal/data"
	"bank-ledger/internal/entity"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"gorm.io/gorm"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
)

var (
	Name     = "transaction-consumer"
	Version  = "v1.0.0"
	flagconf string
	id, _    = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "./configs", "config path, eg: -conf config.yaml")
}

type Transaction struct {
	TransactionID string  `json:"transaction_id"`
	AccountID     string  `json:"account_id"`
	Amount        float64 `json:"amount"`
	Type          string  `json:"type"`
	Description   string  `json:"description"`
	Currency      string  `json:"currency"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
}

type TransactionHandler struct {
	log      *log.Helper
	logger   log.Logger
	confData *conf.Data
}

func (h *TransactionHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *TransactionHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *TransactionHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for message := range claim.Messages() {
		ctx, msgCancel := context.WithTimeout(baseCtx, 10*time.Second)

		h.log.Infof("Received message: topic=%s partition=%d offset=%d", message.Topic, message.Partition, message.Offset)

		var transaction Transaction
		if err := json.Unmarshal(message.Value, &transaction); err != nil {
			h.log.Errorf("Failed to unmarshal transaction: %v", err)
			msgCancel()
			continue
		}

		dataData, cleanup, err := data.NewData(h.confData, h.logger)
		if err != nil {
			h.log.Errorf("Failed to create data: %v", err)
			msgCancel()
			continue
		}

		mongoData, cleanup, err := data.NewMongoDBConnection(h.confData, h.logger)
		if err != nil {
			h.log.Errorf("Failed to create MongoDB connection: %v", err)
			msgCancel()
			continue
		}
		defer cleanup()
		accountRepo := data.NewAccountRepo(dataData, h.logger)
		transactionRepo := data.NewTransactionRepo(dataData, h.logger)
		transactionLogsRepo := data.NewTransactionLogsRepo(dataData, h.logger, mongoData)

		entityTransaction := &entity.Transaction{
			ID:                 transaction.TransactionID,
			AccountID:          transaction.AccountID,
			Amount:             transaction.Amount,
			Currency:           transaction.Currency,
			Description:        transaction.Description,
			Type:               transaction.Type,
			ProcessDescription: "Transaction under process",
			Status:             v1.TransactionStatus_PROCESSING.String(),
			CreatedAt:          parseTimestamp(transaction.CreatedAt),
		}

		err = dataData.DB().Transaction(func(tx *gorm.DB) error {
			ctxTx := context.WithValue(ctx, "tx", tx)

			existingTx, err := transactionRepo.FindByID(ctxTx, &v1.BaseRequest{Id: transaction.TransactionID})
			if err == nil {
				entityTransaction.RetryCount = existingTx.RetryCount + 1
			} else {
				entityTransaction.RetryCount = 1
			}

			if err := transactionRepo.Update(ctxTx, entityTransaction); err != nil {
				return fmt.Errorf("failed to upsert transaction: %w", err)
			}

			if err := transactionLogsRepo.AppendTransactionLog(ctxTx, transaction.TransactionID, entityTransaction.RetryCount, entity.LogEntry{
				Timestamp: time.Now(),
				Message:   "Transaction processing started",
				Status:    v1.TransactionStatus_PROCESSING.String(),
			}); err != nil {
				return fmt.Errorf("failed to append transaction log: %w", err)
			}

			account, err := accountRepo.FindByID(ctxTx, &v1.BaseRequest{Id: transaction.AccountID})
			if err != nil {
				return fmt.Errorf("account not found: %w", err)
			}

			switch transaction.Type {
			case v1.TransactionType_DEPOSIT.String():
				account.Balance += transaction.Amount

			case v1.TransactionType_WITHDRAWAL.String():
				if account.Balance < transaction.Amount {
					return fmt.Errorf("insufficient balance for account: %s", transaction.AccountID)
				}
				account.Balance -= transaction.Amount

			default:
				return fmt.Errorf("unknown transaction type: %s", transaction.Type)
			}

			if err := accountRepo.Update(ctxTx, account); err != nil {
				return fmt.Errorf("failed to update account: %w", err)
			}

			entityTransaction.Status = v1.TransactionStatus_SUCCESS.String()
			entityTransaction.ProcessDescription = "Transaction processed successfully"
			if err := transactionRepo.Update(ctxTx, entityTransaction); err != nil {
				return fmt.Errorf("failed to update transaction to SUCCESS: %w", err)
			}

			if err := transactionLogsRepo.AppendTransactionLog(ctxTx, transaction.TransactionID, entityTransaction.RetryCount, entity.LogEntry{
				Timestamp: time.Now(),
				Message:   "Transaction processed successfully",
				Status:    v1.TransactionStatus_SUCCESS.String(),
			}); err != nil {
				return fmt.Errorf("failed to append transaction log: %w", err)
			} else {
				h.log.Infof("Appended log for transaction %s (try #%d)", transaction.TransactionID, entityTransaction.RetryCount)
			}

			return nil
		})

		if err != nil {
			h.log.Errorf("Transaction processing failed: %v", err)

			if entityTransaction.RetryCount >= 5 {
				entityTransaction.Status = v1.TransactionStatus_FAILED.String()
				entityTransaction.ProcessDescription = err.Error()
			} else {
				entityTransaction.Status = v1.TransactionStatus_PROCESSING.String()
				entityTransaction.ProcessDescription = "Retrying due to error: " + err.Error()
			}

			if updateErr := transactionRepo.Update(ctx, entityTransaction); updateErr != nil {
				h.log.Errorf("Failed to update transaction status: %v", updateErr)
			}

			if logErr := transactionLogsRepo.AppendTransactionLog(ctx, transaction.TransactionID, entityTransaction.RetryCount, entity.LogEntry{
				Timestamp: time.Now(),
				Message:   entityTransaction.ProcessDescription,
				Status:    entityTransaction.Status,
			}); logErr != nil {
				h.log.Errorf("Failed to append transaction log: %v", logErr)
			}

			msgCancel()
			continue
		}

		h.log.Infof("Transaction %s processed successfully", transaction.TransactionID)

		// Mark message as processed
		session.MarkMessage(message, "")
		msgCancel()
	}

	return nil
}

func parseTimestamp(timestamp string) time.Time {
	parsedTime, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return time.Time{}
	}
	return parsedTime
}

func main() {
	flag.Parse()

	logger := log.With(
		log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)

	logHelper := log.NewHelper(logger)

	c := config.New(config.WithSource(file.NewSource(flagconf)))
	defer c.Close()
	if err := c.Load(); err != nil {
		logHelper.Errorf("Failed to load config: %v", err)
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		logHelper.Errorf("Failed to scan config: %v", err)
		panic(err)
	}

	brokers := bc.Data.Kafka.Brokers
	consumerGroup := "transaction-consumer-group"
	topic := "transactions"

	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := sarama.NewConsumerGroup(brokers, consumerGroup, config)
	if err != nil {
		logHelper.Errorf("Error creating consumer group: %v", err)
		return
	}
	defer func() {
		if err := client.Close(); err != nil {
			logHelper.Errorf("Error closing client: %v", err)
		}
	}()

	go func() {
		for err := range client.Errors() {
			logHelper.Errorf("Consumer group error: %v", err)
		}
	}()

	handler := &TransactionHandler{
		log:      logHelper,
		logger:   logger,
		confData: bc.Data,
	}

	go func() {
		for {
			if err := client.Consume(ctx, []string{topic}, handler); err != nil {
				logHelper.Errorf("Error consuming: %v", err)

				if ctx.Err() != nil {
					return
				}

				time.Sleep(5 * time.Second)
			}
		}
	}()

	logHelper.Infof("Transaction consumer started, listening on topic: %s", topic)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logHelper.Info("Shutting down consumer...")
	cancel()

	time.Sleep(2 * time.Second)
}
