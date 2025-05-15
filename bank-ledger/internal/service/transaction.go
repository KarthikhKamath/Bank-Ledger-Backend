package service

import (
	"context"

	v1 "bank-ledger/api/bankLedger/v1"
	"bank-ledger/internal/biz"
)

type TransactionService struct {
	v1.UnimplementedTransactionServer
	trx biz.TransactionHandler
}

func NewTransactionService(trx biz.TransactionHandler) *TransactionService {
	return &TransactionService{trx: trx}
}

func (s *TransactionService) CreateTransaction(ctx context.Context, req *v1.CreateTransactionRequest) (*v1.CreateTransactionResponse, error) {
	transaction, err := s.trx.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	return transaction, nil
}

func (s *TransactionService) GetTransactionById(ctx context.Context, req *v1.GetTransactionByIdRequest) (*v1.GetTransactionResponse, error) {
	transaction, err := s.trx.GetTransactionById(ctx, req)
	if err != nil {
		return nil, err
	}

	return transaction, nil
}

func (s *TransactionService) GetTransactionsByAccount(ctx context.Context, req *v1.GetTransactionsByAccountRequest) (*v1.GetTransactionsByAccountResponse, error) {
	transactions, err := s.trx.GetTransactionsByAccount(ctx, req)
	if err != nil {
		return nil, err
	}

	return transactions, nil
}
