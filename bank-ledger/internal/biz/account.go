package biz

import (
	"bank-ledger/internal/data"
	"bank-ledger/internal/entity"
	"context"
	"fmt"
	"math/rand"
	"time"

	v1 "bank-ledger/api/bankLedger/v1"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/rs/xid"
)

type AccountHandler interface {
	Create(ctx context.Context, req *v1.CreateAccountRequest) (*v1.AccountResponse, error)
	Update(ctx context.Context, req *v1.UpdateAccountRequest) (*v1.AccountResponse, error)
	FindByID(ctx context.Context, req *v1.BaseRequest) (*v1.AccountResponse, error)
	ListAll(ctx context.Context) ([]*v1.AccountResponse, error)
	Delete(ctx context.Context, req *v1.BaseRequest) error
}

type Account struct {
	repo data.AccountRepository
	log  *log.Helper
}

func NewAccountHandler(repo data.AccountRepository, logger log.Logger) AccountHandler {
	return &Account{repo: repo, log: log.NewHelper(logger)}
}

func generateAccountNumber() string {
	var rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("%012d", rng.Int63n(1000000000000))
}

func (uc *Account) Create(ctx context.Context, req *v1.CreateAccountRequest) (*v1.AccountResponse, error) {
	id := xid.New().String()
	acc := &entity.Account{
		ID:            id,
		AccountNumber: generateAccountNumber(),
		Name:          req.Name,
		Currency:      req.Currency.String(),
		Balance:       0.0,
		Status:        v1.AccountStatus_ACTIVE.String(),
	}

	if err := uc.repo.Create(ctx, acc); err != nil {
		return nil, err
	}

	return toProtoAccount(acc), nil
}

func (uc *Account) Update(ctx context.Context, req *v1.UpdateAccountRequest) (*v1.AccountResponse, error) {
	acc, err := uc.repo.FindByID(ctx, &v1.BaseRequest{Id: req.Id})
	if err != nil {
		return nil, err
	}

	acc = &entity.Account{
		ID:            req.Id,
		Name:          req.Name,
		AccountNumber: acc.AccountNumber,
		Status:        req.Status.String(),
		CreatedAt:     acc.CreatedAt,
		UpdatedAt:     time.Now(),
	}
	if err := uc.repo.Update(ctx, acc); err != nil {
		return nil, err
	}
	return toProtoAccount(acc), nil
}

func (uc *Account) FindByID(ctx context.Context, req *v1.BaseRequest) (*v1.AccountResponse, error) {
	acc, err := uc.repo.FindByID(ctx, req)
	if err != nil {
		return nil, err
	}
	return toProtoAccount(acc), nil
}

func (uc *Account) ListAll(ctx context.Context) ([]*v1.AccountResponse, error) {
	accs, err := uc.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	resp := make([]*v1.AccountResponse, 0, len(accs))
	for _, acc := range accs {
		resp = append(resp, toProtoAccount(acc))
	}
	return resp, nil
}

func (uc *Account) Delete(ctx context.Context, req *v1.BaseRequest) error {
	acc, err := uc.repo.FindByID(ctx, req)
	if err != nil {
		return err
	}

	acc = &entity.Account{
		ID:        req.Id,
		Name:      acc.Name,
		Status:    v1.AccountStatus_CLOSED.String(),
		CreatedAt: acc.CreatedAt,
		UpdatedAt: time.Now(),
	}
	if err = uc.repo.Update(ctx, acc); err != nil {
		return err
	}
	return nil
}

func toProtoAccount(acc *entity.Account) *v1.AccountResponse {
	return &v1.AccountResponse{
		Id:            acc.ID,
		AccountNumber: acc.AccountNumber,
		Name:          acc.Name,
		Balance:       formatFloat(acc.Balance),
		Currency:      v1.Currency(v1.Currency_value[acc.Currency]),
		Status:        v1.AccountStatus(v1.AccountStatus_value[acc.Status]),
		CreatedAt:     acc.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     acc.UpdatedAt.Format(time.RFC3339),
	}
}

func formatFloat(val float64) string {
	return fmt.Sprintf("%.2f", val)
}
