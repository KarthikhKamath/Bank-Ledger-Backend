package data

import (
	v1 "bank-ledger/api/bankLedger/v1"
	"bank-ledger/internal/entity"
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type AccountRepository interface {
	Create(ctx context.Context, req *entity.Account) error
	Update(ctx context.Context, req *entity.Account) error
	FindByID(ctx context.Context, req *v1.BaseRequest) (*entity.Account, error)
	ListAll(ctx context.Context) ([]*entity.Account, error)
	Delete(ctx context.Context, req *v1.BaseRequest) error
	WithTx(tx *gorm.DB) AccountRepository
}

type AccountRepo struct {
	data *Data
	db   *gorm.DB
	log  *log.Helper
}

func NewAccountRepo(data *Data, logger log.Logger) AccountRepository {
	return &AccountRepo{
		data: data,
		db:   data.db,
		log:  log.NewHelper(logger),
	}
}

func (r *AccountRepo) WithTx(tx *gorm.DB) AccountRepository {
	return &AccountRepo{
		data: r.data,
		db:   tx,
		log:  r.log,
	}
}

func (r *AccountRepo) Create(ctx context.Context, req *entity.Account) error {
	if err := r.db.WithContext(ctx).Create(req).Error; err != nil {
		return err
	}
	return nil
}

func (r *AccountRepo) Update(ctx context.Context, req *entity.Account) error {
	req.UpdatedAt = time.Now()
	if err := r.db.WithContext(ctx).Save(req).Error; err != nil {
		return err
	}
	return nil
}

func (r *AccountRepo) FindByID(ctx context.Context, req *v1.BaseRequest) (*entity.Account, error) {
	var account entity.Account
	if err := r.db.WithContext(ctx).First(&account, "id = ?", req.Id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *AccountRepo) ListAll(ctx context.Context) ([]*entity.Account, error) {
	var accounts []*entity.Account
	if err := r.db.WithContext(ctx).Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *AccountRepo) Delete(ctx context.Context, req *v1.BaseRequest) error {
	return r.db.WithContext(ctx).Delete(&entity.Account{}, "id = ?", req.Id).Error
}
