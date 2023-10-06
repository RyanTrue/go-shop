package main

import (
	"context"
	"database/sql"
	"github.com/RyanTrue/go-shop/cmd/internal/app/config"
	"github.com/RyanTrue/go-shop/cmd/internal/app/handlers"
	zaplogger "github.com/RyanTrue/go-shop/cmd/internal/app/logger"
	"github.com/RyanTrue/go-shop/cmd/internal/repository"
	"github.com/RyanTrue/go-shop/cmd/internal/services"
	_ "github.com/jackc/pgx/v5/stdlib"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	//logger, conf, db inits
	logger, err := zaplogger.InitLogger("Info")
	if err != nil {
		panic(err)
	}

	conf, err := config.NewConfig()
	if err != nil {
		logger.Fatal(err)
	}

	db, err := sql.Open("pgx", conf.DatabaseURI)
	if err != nil {
		logger.Fatal(err)
	}
	defer db.Close()

	err = repository.InitDB(db)
	if err != nil {
		logger.Fatal(err)
	}

	//dependency injections
	repo := repository.NewDBStorage(db)
	authService := services.NewAuthService(repo, logger)
	ordersService := services.NewOrderService(repo, logger)
	ordersProcessingService := services.NewOrderProcessor(repo, conf.AccrualSystemAddress, logger)
	handler := handlers.NewHandler(authService, ordersService, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalCh
		cancel()
	}()

	go ordersProcessingService.ProcessOrders(ctx)

	//router setup
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(echojwt.WithConfig(echojwt.Config{
		SigningKey:  []byte(conf.JWTKey),
		TokenLookup: "cookie:jwt",
		Skipper: func(c echo.Context) bool {
			if c.Request().URL.Path == "/api/user/register" || c.Request().URL.Path == "/api/user/login" {
				return true
			}
			return false
		},
	}))

	e.POST("/api/user/register", handler.Register)
	e.POST("/api/user/login", handler.Login)
	e.GET("/api/user/orders", handler.GetOrders)
	e.POST("/api/user/orders", handler.UploadOrder)
	e.GET("/api/user/balance", handler.GetBalance)
	e.POST("/api/user/balance/withdraw", handler.Withdraw)
	e.GET("/api/user/withdrawals", handler.GetWithdrawals)

	go func() {
		if err := e.Start(conf.RunAddress); err != nil {
			logger.Error("Failed to start server: ", err)
			cancel()
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Failed to gracefully shut down the server: ", err)
	}

}
