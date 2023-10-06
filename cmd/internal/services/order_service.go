package services

import (
	"context"
	"github.com/RyanTrue/go-shop/cmd/internal/app/models"
	"github.com/RyanTrue/go-shop/cmd/internal/repository"
	"go.uber.org/zap"
)

type OrderService interface {
	GetUsersOrders(ctx context.Context, useLogin string) ([]models.Order, error)
	UploadOrder(ctx context.Context, userLogin string, orderNumber string) (bool, error)
	GetBalance(ctx context.Context, userLogin string) (models.AccountBalance, error)
	Withdrawals(ctx context.Context, userLogin string, withdraw models.WithDrawRequest) error
	GetUsersWithdrawals(ctx context.Context, userLogin string) ([]models.Withdraw, error)
}

type orderService struct {
	Repo   repository.Repository
	logger *zap.SugaredLogger
}

func NewOrderService(repo repository.Repository, logger *zap.SugaredLogger) OrderService {
	return &orderService{
		Repo:   repo,
		logger: logger,
	}
}

func (o *orderService) GetUsersOrders(ctx context.Context, userLogin string) ([]models.Order, error) {

	orders, err := o.Repo.GetUsersOrders(ctx, userLogin)
	if err != nil {
		return nil, err
	}

	return orders, nil
}

func (o *orderService) UploadOrder(ctx context.Context, userLogin string, orderNumber string) (bool, error) {

	existing, err := o.Repo.UploadOrder(ctx, userLogin, orderNumber)

	return existing, err
}

func (o *orderService) GetBalance(ctx context.Context, userLogin string) (models.AccountBalance, error) {

	balance, err := o.Repo.GetBalance(ctx, userLogin)
	if err != nil {
		return models.AccountBalance{}, err
	}

	return balance, nil

}

func (o *orderService) Withdrawals(ctx context.Context, userLogin string, withdraw models.WithDrawRequest) error {

	err := o.Repo.Withdrawal(ctx, userLogin, withdraw)
	if err != nil {
		return err
	}

	return nil
}

func (o *orderService) GetUsersWithdrawals(ctx context.Context, userLogin string) ([]models.Withdraw, error) {

	withdraws, err := o.Repo.GetUsersWithdrawals(ctx, userLogin)
	if err != nil {
		return nil, err
	}

	return withdraws, nil
}
