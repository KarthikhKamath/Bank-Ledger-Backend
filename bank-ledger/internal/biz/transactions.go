package biz

import (
	"bank-ledger/internal/data"
	"bank-ledger/internal/entity"
	"context"
	"encoding/json"
	"github.com/go-kratos/kratos/v2/errors"
	"net/http"
	"time"

	v1 "bank-ledger/api/bankLedger/v1"
	"bank-ledger/internal/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/rs/xid"
)

type TransactionHandler interface {
	Create(ctx context.Context, req *v1.CreateTransactionRequest) (*v1.CreateTransactionResponse, error)
	GetTransactionById(ctx context.Context, req *v1.GetTransactionByIdRequest) (*v1.GetTransactionResponse, error)
	GetTransactionsByAccount(ctx context.Context, req *v1.GetTransactionsByAccountRequest) (*v1.GetTransactionsByAccountResponse, error)
}

type Transaction struct {
	log      *log.Helper
	producer kafka.Producer
	acc      data.AccountRepository
	trx      data.TransactionRepository
	trxLog   data.TransactionLogsRepository
}

func NewTransactionHandler(producer kafka.Producer, logger log.Logger, acc data.AccountRepository, trx data.TransactionRepository, trxLogs data.TransactionLogsRepository) TransactionHandler {
	return &Transaction{
		producer: producer,
		log:      log.NewHelper(logger),
		acc:      acc,
		trx:      trx,
		trxLog:   trxLogs,
	}
}

func (t *Transaction) Create(ctx context.Context, req *v1.CreateTransactionRequest) (*v1.CreateTransactionResponse, error) {

	if req.AccountId == "" {
		return nil, errors.New(http.StatusNotFound, http.StatusText(http.StatusNotFound), "account_id is required")
	}

	acc, err := t.acc.FindByID(ctx, &v1.BaseRequest{Id: req.AccountId})
	if err != nil {
		return nil, errors.New(http.StatusNotFound, http.StatusText(http.StatusNotFound), "account does not exist")
	}

	if acc.Status == v1.AccountStatus_CLOSED.String() {
		return nil, errors.New(http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "account is closed")
	}

	if req.Type == v1.TransactionType_WITHDRAWAL && acc.Balance < req.Amount {
		return nil, errors.New(http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "insufficient balance")
	}

	transactionID := xid.New().String()

	now := time.Now()
	createdAt := now.Format(time.RFC3339)
	if err := t.trx.Create(ctx, &entity.Transaction{
		ID:          transactionID,
		AccountID:   req.AccountId,
		Amount:      req.Amount,
		Type:        req.Type.String(),
		Description: req.Description,
		Currency:    acc.Currency,
		Status:      v1.TransactionStatus_INITIATED.String(),
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		return nil, errors.New(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "failed to create transaction")
	}

	err = t.trxLog.CreateTransaction(ctx, &entity.TransactionLog{
		TransactionID: transactionID,
		Timestamp:     now,
		Message:       "Transaction initiated",
		Status:        v1.TransactionStatus_INITIATED.String(),
		TryCount:      0,
		TransactionLogs: []entity.TryLog{
			{
				Attempt:   1,
				Timestamp: now,
				Logs: []entity.LogEntry{
					{
						Timestamp: now,
						Message:   "Transaction initiated and event published to Kafka successfully",
						Status:    v1.TransactionStatus_INITIATED.String(),
					},
				},
			},
		},
	})
	if err != nil {
		t.log.Errorf("failed to create transaction log in MongoDB: %v", err)
		// Optional: Decide whether to fail or continue. Here we continue.
	}

	kafkaMessage := map[string]interface{}{
		"transaction_id": transactionID,
		"account_id":     req.AccountId,
		"amount":         req.Amount,
		"type":           req.Type.String(),
		"description":    req.Description,
		"currency":       acc.Currency,
		"status":         v1.TransactionStatus_INITIATED.String(),
		"created_at":     createdAt,
	}

	messageBytes, err := json.Marshal(kafkaMessage)
	if err != nil {
		t.log.Errorf("failed to marshal transaction for kafka: %v", err)
		return nil, err
	}

	err = t.producer.SendMessage("transactions", []byte(transactionID), messageBytes)
	if err != nil {
		t.log.Errorf("failed to publish transaction to kafka: %v", err)
		return nil, err
	}

	return &v1.CreateTransactionResponse{
		TransactionId: transactionID,
		AccountId:     req.AccountId,
		Status:        v1.TransactionStatus_INITIATED,
		CreatedAt:     createdAt,
	}, nil
}

func (t *Transaction) GetTransactionById(ctx context.Context, req *v1.GetTransactionByIdRequest) (*v1.GetTransactionResponse, error) {
	if req.TransactionId == "" {
		return nil, errors.New(400, "BAD_REQUEST", "transaction_id is required")
	}

	trx, err := t.trx.FindByID(ctx, &v1.BaseRequest{Id: req.TransactionId})
	if err != nil {
		return nil, errors.NotFound("TRANSACTION_NOT_FOUND", "transaction not found")
	}

	logs, _ := t.trxLog.GetTransaction(ctx, req.TransactionId)

	var protoLogs []*v1.TransactionLog
	if logs != nil {
		for _, try := range logs.TransactionLogs {
			for _, l := range try.Logs {
				protoLogs = append(protoLogs, &v1.TransactionLog{
					Timestamp: l.Timestamp.Format(time.RFC3339),
					Message:   l.Message,
					Status:    l.Status,
					Attempt:   int32(try.Attempt),
				})
			}
		}
	}

	return &v1.GetTransactionResponse{
		Transaction: &v1.EachTransaction{
			Id:          trx.ID,
			AccountId:   trx.AccountID,
			Amount:      trx.Amount,
			Type:        v1.TransactionType(v1.TransactionType_value[trx.Type]),
			Description: trx.Description,
			Currency:    trx.Currency,
			Status:      v1.TransactionStatus(v1.TransactionStatus_value[trx.Status]),
			CreatedAt:   trx.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   trx.UpdatedAt.Format(time.RFC3339),
		},
		Logs: protoLogs,
	}, nil
}

func (t *Transaction) GetTransactionsByAccount(ctx context.Context, req *v1.GetTransactionsByAccountRequest) (*v1.GetTransactionsByAccountResponse, error) {
	if req.AccountId == "" {
		return nil, errors.BadRequest("ACCOUNT_ID_REQUIRED", "account_id is required")
	}

	offset := (req.Page - 1) * req.PageSize
	limit := req.PageSize

	trxs, total, err := t.trx.FindByAccountIDWithPagination(ctx, req.AccountId, int(offset), int(limit))
	if err != nil {
		return nil, errors.InternalServer("DB_ERROR", err.Error())
	}

	account, _ := t.acc.FindByID(ctx, &v1.BaseRequest{Id: req.AccountId})

	var result []*v1.EachTransaction
	for _, tx := range trxs {
		result = append(result, &v1.EachTransaction{
			Id:          tx.ID,
			AccountId:   tx.AccountID,
			Amount:      tx.Amount,
			Type:        v1.TransactionType(v1.TransactionType_value[tx.Type]),
			Description: tx.Description,
			Currency:    tx.Currency,
			Status:      v1.TransactionStatus(v1.TransactionStatus_value[tx.Status]),
			CreatedAt:   tx.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   tx.UpdatedAt.Format(time.RFC3339),
		})
	}

	totalPages := (int32(total) + req.PageSize - 1) / req.PageSize

	return &v1.GetTransactionsByAccountResponse{
		AccountId:    req.AccountId,
		Transactions: result,
		Pagination: &v1.PaginationInfo{
			TotalCount: int32(total),
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		AccountInfo: &v1.AccountInfo{
			Id:       account.ID,
			Balance:  account.Balance,
			Currency: account.Currency,
			Status:   account.Status,
		},
	}, nil
}
