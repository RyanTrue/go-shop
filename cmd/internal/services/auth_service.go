package services

import (
	"context"
	"github.com/RyanTrue/go-shop/cmd/internal/app/models"
	"github.com/RyanTrue/go-shop/cmd/internal/repository"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Login(ctx context.Context, credentials models.Credentials) error
	Register(ctx context.Context, credentials models.Credentials) error
}

type authService struct {
	Repo   repository.Repository
	logger *zap.SugaredLogger
}

func NewAuthService(repo repository.Repository, logger *zap.SugaredLogger) AuthService {
	return &authService{
		Repo:   repo,
		logger: logger,
	}
}

func (a *authService) Login(ctx context.Context, credentials models.Credentials) error {

	hashedPass, err := a.Repo.Login(ctx, credentials.Login)
	if err != nil {
		return err
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPass), []byte(credentials.Password))
	if err != nil {
		a.logger.Errorw("Could not compare hashed password", "error", err)
		return err
	}

	return nil
}

func (a *authService) Register(ctx context.Context, credentials models.Credentials) error {

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(credentials.Password), bcrypt.DefaultCost)
	if err != nil {
		a.logger.Errorw("Could not generate hashed password", "error", err)
		return err
	}

	err = a.Repo.Register(ctx, credentials.Login, string(hashedPass))
	if err != nil {
		return err
	}

	return nil

}
