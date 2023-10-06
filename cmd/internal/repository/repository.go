package repository

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/RyanTrue/go-shop/cmd/internal/app/models"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	errors2 "github.com/pkg/errors"
	"time"
)

type Repository interface {
	Login(ctx context.Context, login string) (string, error)
	Register(ctx context.Context, login string, passwords string) error
	GetUsersOrders(ctx context.Context, userLogin string) ([]models.Order, error)
	UploadOrder(ctx context.Context, userLogin string, orderNumber string) (bool, error)
	GetBalance(ctx context.Context, userLogin string) (models.AccountBalance, error)
	Withdrawal(ctx context.Context, userLogin string, withdraw models.WithDrawRequest) error
	GetUsersWithdrawals(ctx context.Context, userLogin string) ([]models.Withdraw, error)
	GetNewOrders(ctx context.Context) ([]models.Order, error)
	UpdateOrderStatus(ctx context.Context, orderNumber string, status string, accrual float64) error
	SetOrderStatusInvalid(ctx context.Context, orderNumber string) error
	GetStaleProcessingOrders(ctx context.Context, staleThreshold time.Duration) ([]models.Order, error)
}

type dbStorage struct {
	db *sql.DB
}

func NewDBStorage(db *sql.DB) Repository {
	return &dbStorage{db: db}
}

func InitDB(db *sql.DB) error {

	//start transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS users(login varchar(20) primary key UNIQUE, password varchar(100), current_balance float, withdrawn float)")
	if err != nil {
		return errors2.Wrap(err, "Could not create users table on db init")
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS orders(order_number varchar(30) primary key UNIQUE, status varchar(20), accrual float, uploaded_at timestamp, last_changed_at timestamp, login_users varchar(20) REFERENCES users(login))")
	if err != nil {
		return errors2.Wrap(err, "Could not create orders table on db init")
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS withdraws(order_num varchar(30) primary key UNIQUE, sum float, processed_at timestamp, login_users varchar(30) REFERENCES users(login))")
	if err != nil {
		return errors2.Wrap(err, "Could not create withdraws table on db init")
	}

	return tx.Commit()
}

func (s *dbStorage) Login(ctx context.Context, login string) (string, error) {

	ctrl, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	//get hashed password from db
	var hashedPass string
	err := s.db.QueryRowContext(ctrl, "SELECT password FROM users WHERE login = $1", login).Scan(&hashedPass)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors2.Wrap(err, "user does not exist")
		}
		return "", err
	}

	return hashedPass, nil
}

func (s *dbStorage) Register(ctx context.Context, login string, passwords string) error {

	ctrl, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	_, err := s.db.ExecContext(ctrl, "INSERT INTO users (login, password, current_balance, withdrawn) VALUES ($1, $2, 0, 0)", login, passwords)
	if err != nil {
		if err, ok := err.(*pgconn.PgError); ok && err.Code == pgerrcode.UniqueViolation {
			return errors2.New("user already exists")
		}
		return err
	}
	return err

}

func (s *dbStorage) GetUsersOrders(ctx context.Context, userLogin string) ([]models.Order, error) {

	ctrl, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	var orders []models.Order

	rows, err := s.db.QueryContext(ctrl, `SELECT order_number, status, accrual, uploaded_at, last_changed_at FROM orders WHERE "login_users" = $1 ORDER BY uploaded_at DESC`, userLogin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var order models.Order
		err = rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt, &order.LastChangedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func (s *dbStorage) UploadOrder(ctx context.Context, userLogin string, orderNumber string) (bool, error) {
	ctrl, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	_, err := s.db.ExecContext(ctrl, `INSERT INTO orders(order_number, status, accrual, uploaded_at, last_changed_at, login_users) VALUES($1, 'NEW' , 0, NOW(), NOW(), $2)`, orderNumber, userLogin)
	if err != nil {
		// Check if order number is already in the database
		var existingUserLogin string
		err2 := s.db.QueryRowContext(ctrl, `SELECT login_users FROM orders WHERE order_number = $1`, orderNumber).Scan(&existingUserLogin)
		if err2 == nil {
			if existingUserLogin == userLogin {
				// Order exists and was uploaded by the current user
				return true, nil
			}
			// Order exists but was uploaded by another user
			return false, fmt.Errorf("order already exists by another user")
		}
		return false, err
	}
	return false, nil
}

func (s *dbStorage) GetBalance(ctx context.Context, userLogin string) (models.AccountBalance, error) {

	ctrl, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	var balance models.AccountBalance

	err := s.db.QueryRowContext(ctrl, `SELECT current_balance, withdrawn FROM users WHERE login = $1`, userLogin).Scan(&balance.CurrentBalance, &balance.Withdrawn)
	if err != nil {
		return models.AccountBalance{}, err
	}

	return balance, nil
}

func (s *dbStorage) Withdrawal(ctx context.Context, userLogin string, withdraw models.WithDrawRequest) error {

	ctrl, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	tx, err := s.db.BeginTx(ctrl, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = s.db.ExecContext(ctrl, `UPDATE users SET current_balance = current_balance - $1, withdrawn = withdrawn + $1 WHERE login = $2`, withdraw.Sum, userLogin)
	if err != nil {
		if err, ok := err.(*pgconn.PgError); ok && err.Code == pgerrcode.CheckViolation {
			return errors2.New("not enough money")
		}
		return err
	}

	_, err = s.db.ExecContext(ctrl, `INSERT INTO withdraws (order_num, sum, processed_at,login_users) VALUES ($1, $2, NOW(), $3)`, withdraw.OrderNumber, withdraw.Sum, userLogin)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *dbStorage) GetUsersWithdrawals(ctx context.Context, userLogin string) ([]models.Withdraw, error) {

	ctrl, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	var withdraws []models.Withdraw

	rows, err := s.db.QueryContext(ctrl, `SELECT order_num, sum, processed_at FROM withdraws WHERE "login_users" = $1 ORDER BY processed_at DESC`, userLogin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var withdraw models.Withdraw
		err = rows.Scan(&withdraw.OrderNumber, &withdraw.Sum, &withdraw.ProcessedAt)
		if err != nil {
			return nil, err
		}
		withdraws = append(withdraws, withdraw)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return withdraws, nil
}

func (s *dbStorage) GetNewOrders(ctx context.Context) ([]models.Order, error) {

	ctrl, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	var orders []models.Order

	//query to get new orders using update returning clause and change status to "PROCESSING"
	rows, err := s.db.QueryContext(ctrl, `UPDATE orders SET status = 'PROCESSING' WHERE status = 'NEW' RETURNING order_number, status, accrual, uploaded_at`)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var order models.Order
		err = rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}

func (s *dbStorage) UpdateOrderStatus(ctx context.Context, orderNumber string, status string, accrual float64) error {

	ctrl, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = s.db.ExecContext(ctrl, `UPDATE orders SET status = $1, accrual = $2 WHERE order_number = $3`, status, accrual, orderNumber)
	if err != nil {
		return err
	}

	//update user balance
	_, err = s.db.ExecContext(ctrl, `UPDATE users SET current_balance = current_balance + $1 WHERE login = (SELECT login_users FROM orders WHERE order_number = $2)`, accrual, orderNumber)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *dbStorage) SetOrderStatusInvalid(ctx context.Context, orderNumber string) error {

	ctrl, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	_, err := s.db.ExecContext(ctrl, `UPDATE orders SET status = 'INVALID' WHERE order_number = $1`, orderNumber)
	if err != nil {
		return err
	}

	return nil
}

func (s *dbStorage) GetStaleProcessingOrders(ctx context.Context, staleThreshold time.Duration) ([]models.Order, error) {

	ctrl, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	var orders []models.Order

	rows, err := s.db.QueryContext(ctrl, `SELECT order_number, status, accrual, uploaded_at, last_changed_at FROM orders WHERE status = 'PROCESSING' AND last_changed_at < NOW() - $1`, staleThreshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var order models.Order
		err = rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt, &order.LastChangedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}
