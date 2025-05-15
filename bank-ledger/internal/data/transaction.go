package data

import (
	v1 "bank-ledger/api/bankLedger/v1"
	"bank-ledger/internal/entity"
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type TransactionRepository interface {
	Create(ctx context.Context, req *entity.Transaction) error
	Update(ctx context.Context, req *entity.Transaction) error
	FindByID(ctx context.Context, req *v1.BaseRequest) (*entity.Transaction, error)
	ListAll(ctx context.Context) ([]*entity.Transaction, error)
	FindByAccountIDWithPagination(ctx context.Context, accountID string, offset int, limit int) ([]*entity.Transaction, int64, error)
	WithTx(tx *gorm.DB) TransactionRepository
}

type TransactionRepo struct {
	data *Data
	db   *gorm.DB
	log  *log.Helper
}

func NewTransactionRepo(data *Data, logger log.Logger) TransactionRepository {
	return &TransactionRepo{
		data: data,
		db:   data.db,
		log:  log.NewHelper(logger),
	}
}

func (r *TransactionRepo) WithTx(tx *gorm.DB) TransactionRepository {
	return &TransactionRepo{
		data: r.data,
		db:   tx,
		log:  r.log,
	}
}

func (r *TransactionRepo) Create(ctx context.Context, req *entity.Transaction) error {
	if err := r.db.WithContext(ctx).Create(req).Error; err != nil {
		return err
	}
	return nil
}

func (r *TransactionRepo) Update(ctx context.Context, req *entity.Transaction) error {
	req.UpdatedAt = time.Now()
	if err := r.db.WithContext(ctx).Save(req).Error; err != nil {
		return err
	}
	return nil
}

func (r *TransactionRepo) FindByID(ctx context.Context, req *v1.BaseRequest) (*entity.Transaction, error) {
	var txn entity.Transaction
	if err := r.db.WithContext(ctx).First(&txn, "id = ?", req.Id).Error; err != nil {
		return nil, err
	}
	return &txn, nil
}

func (r *TransactionRepo) ListAll(ctx context.Context) ([]*entity.Transaction, error) {
	var transactions []*entity.Transaction
	if err := r.db.WithContext(ctx).Find(&transactions).Error; err != nil {
		return nil, err
	}
	return transactions, nil
}

func (r *TransactionRepo) FindByAccountIDWithPagination(ctx context.Context, accountID string, offset int, limit int) ([]*entity.Transaction, int64, error) {
	var transactions []*entity.Transaction
	var total int64

	query := r.db.WithContext(ctx).Model(&entity.Transaction{}).Where("account_id = ?", accountID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&transactions).Error; err != nil {
		return nil, 0, err
	}

	return transactions, total, nil
}
