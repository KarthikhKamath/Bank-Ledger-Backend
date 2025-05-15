package service

import (
	"context"

	v1 "bank-ledger/api/bankLedger/v1"
	"bank-ledger/internal/biz"
)

type AccountService struct {
	v1.UnimplementedAccountServer
	uc biz.AccountHandler
}

func NewAccountService(uc biz.AccountHandler) *AccountService {
	return &AccountService{uc: uc}
}

func (s *AccountService) CreateAccount(ctx context.Context, req *v1.CreateAccountRequest) (*v1.AccountResponse, error) {
	createAcc, err := s.uc.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return createAcc, nil
}

func (s *AccountService) GetAccount(ctx context.Context, req *v1.BaseRequest) (*v1.AccountResponse, error) {
	account, err := s.uc.FindByID(ctx, req)
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (s *AccountService) GetAllAccounts(ctx context.Context, req *v1.EmptyRequest) (*v1.GetAllAccountsResponse, error) {
	accounts, err := s.uc.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	return &v1.GetAllAccountsResponse{
		Accounts: accounts,
	}, nil
}

func (s *AccountService) UpdateAccount(ctx context.Context, req *v1.UpdateAccountRequest) (*v1.AccountResponse, error) {
	accountResp, err := s.uc.Update(ctx, req)
	if err != nil {
		return nil, err
	}
	return accountResp, nil
}

func (s *AccountService) DeleteAccount(ctx context.Context, req *v1.BaseRequest) (*v1.DeleteAccountResponse, error) {
	err := s.uc.Delete(ctx, req)
	if err != nil {
		return nil, err
	}
	return &v1.DeleteAccountResponse{Success: true}, nil
}
