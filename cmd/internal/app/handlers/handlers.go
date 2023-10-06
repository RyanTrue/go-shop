package handlers

import (
	"encoding/json"
	"github.com/RyanTrue/go-shop/cmd/internal/app/models"
	"github.com/RyanTrue/go-shop/cmd/internal/services"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	errors2 "github.com/pkg/errors"
	"go.uber.org/zap"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Handler struct {
	authService   services.AuthService
	ordersService services.OrderService
	logger        *zap.SugaredLogger
}

func NewHandler(authService services.AuthService, ordersSerive services.OrderService, logger *zap.SugaredLogger) *Handler {
	return &Handler{
		authService:   authService,
		ordersService: ordersSerive,
		logger:        logger,
	}
}

func (h *Handler) Register(c echo.Context) error {

	var cred models.Credentials

	err := json.NewDecoder(c.Request().Body).Decode(&cred)
	if err != nil {
		h.logger.Errorw("Could not decode credentials from request body", "error", err)
		return err
	}

	if cred.Login == "" || cred.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "bad request"})
	}

	err = h.authService.Register(c.Request().Context(), cred)
	if err != nil {
		if err.Error() == "user already exists" {
			return c.JSON(http.StatusConflict, map[string]string{"message": "user already exists"})
		}
		h.logger.Errorw("Could not register user", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "internal server error"})
	}

	err = setJWTCookie(c, cred.Login)
	if err != nil {
		h.logger.Errorw("Could not set jwt cookie", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "internal server error"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "User registered successfully"})

}

func (h *Handler) Login(c echo.Context) error {

	var cred models.Credentials

	err := json.NewDecoder(c.Request().Body).Decode(&cred)
	if err != nil {
		h.logger.Errorw("Could not decode credentials from request body", "error", err)
		return err
	}

	if cred.Login == "" || cred.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "bad request"})
	}

	err = h.authService.Login(c.Request().Context(), cred)
	if err != nil {
		if errors2.Unwrap(err).Error() == "user not found" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"message": "user not found"})
		}
		h.logger.Errorw("Could not login user", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "internal server error"})
	}

	err = setJWTCookie(c, cred.Login)
	if err != nil {
		h.logger.Errorw("Could not set jwt cookie", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "internal server error"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "User logged in successfully"})
}

func (h *Handler) UploadOrder(c echo.Context) error {

	var orderNumber int
	err := json.NewDecoder(c.Request().Body).Decode(&orderNumber)
	if err != nil {
		h.logger.Errorw("Could not decode order number from request body", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "bad request"})
	}

	if !isValidLuhn(orderNumber) {
		h.logger.Errorw("Invalid order number", "error", err)
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"message": "bad request"})
	}

	userLogin, err := getUserLoginFromToken(c)
	if err != nil {
		h.logger.Errorw("Could not get user login from token", "error", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "internal server error"})
	}
	existing, err := h.ordersService.UploadOrder(c.Request().Context(), userLogin, strconv.Itoa(orderNumber))
	if err != nil {
		if err.Error() == "order already exists by another user" {
			return c.JSON(http.StatusConflict, map[string]string{"message": "Order already uploaded by another user"})
		}
		h.logger.Errorw("Could not upload order", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "internal server error"})
	}
	if existing {
		return c.JSON(http.StatusOK, map[string]string{"message": "Order already uploaded by the current user"})
	}
	return c.JSON(http.StatusAccepted, map[string]string{"message": "Order uploaded successfully"})
}

func (h *Handler) GetOrders(c echo.Context) error {

	userLogin, err := getUserLoginFromToken(c)
	if err != nil {
		h.logger.Errorw("Could not get user login from token", "error", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "internal server error"})
	}
	usersOrders, err := h.ordersService.GetUsersOrders(c.Request().Context(), userLogin)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Internal server error"})
	}
	if len(usersOrders) == 0 {
		return c.JSON(http.StatusNoContent, map[string]string{"message": "No orders"})
	}

	return c.JSON(http.StatusOK, usersOrders)
}

func (h *Handler) GetBalance(c echo.Context) error {

	userLogin, err := getUserLoginFromToken(c)
	if err != nil {
		h.logger.Errorw("Could not get user login from token", "error", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "internal server error"})
	}
	balance, err := h.ordersService.GetBalance(c.Request().Context(), userLogin)
	if err != nil {
		h.logger.Errorw("Could not get balance", "error", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "Internal server error"})
	}

	return c.JSON(http.StatusOK, balance)

}

func (h *Handler) Withdraw(c echo.Context) error {

	var withdraw models.WithDrawRequest
	err := json.NewDecoder(c.Request().Body).Decode(&withdraw)
	if err != nil {
		h.logger.Errorw("Could not decode withdraw request from request body", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "bad request"})
	}
	userLogin, err := getUserLoginFromToken(c)
	if err != nil {
		h.logger.Errorw("Could not get user login from token", "error", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "internal server error"})
	}

	err = h.ordersService.Withdrawals(c.Request().Context(), userLogin, withdraw)
	if err != nil {
		if errors2.Unwrap(err).Error() == "not enough money" {
			return c.JSON(http.StatusPaymentRequired, map[string]string{"message": "not enough money"})
		}
		h.logger.Errorw("Could not withdraw", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Internal server error"})
	}

	return nil

}

func (h *Handler) GetWithdrawals(c echo.Context) error {

	userLogin, err := getUserLoginFromToken(c)
	if err != nil {
		h.logger.Errorw("Could not get user login from token", "error", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "internal server error"})
	}

	withdraws, err := h.ordersService.GetUsersWithdrawals(c.Request().Context(), userLogin)
	if err != nil {
		h.logger.Errorw("Could not get withdrawals", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Internal server error"})
	}

	if len(withdraws) == 0 {
		return c.JSON(http.StatusNoContent, map[string]string{"message": "No withdrawals"})
	}

	return c.JSON(http.StatusOK, withdraws)
}

func isValidLuhn(num int) bool {
	var sum int

	s := strconv.Itoa(num) // Convert the integer to a string to handle each digit
	parity := len(s) % 2

	for i, c := range s {
		digit, err := strconv.Atoi(string(c))
		if err != nil {
			return false // not a digit, should not happen but kept for completeness
		}

		if i%2 == parity {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}

		sum += digit
	}

	return sum%10 == 0
}

func generateJWTToken(userLogin string) (string, error) {

	claims := &models.JwtCustomClaims{
		Login: userLogin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 1)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtKey := os.Getenv("JWT_KEY")
	return token.SignedString([]byte(jwtKey))

}

func setJWTCookie(c echo.Context, login string) error {
	token, err := generateJWTToken(login)
	if err != nil {
		return err
	}

	cookie := &http.Cookie{
		Name:     "jwt",
		Value:    token,
		Expires:  time.Now().Add(time.Hour * 1),
		HttpOnly: true,
	}
	c.SetCookie(cookie)
	return nil
}

func getUserLoginFromToken(c echo.Context) (string, error) {
	//get user login from jwt token
	cookie, err := c.Cookie("jwt")
	if err != nil {
		return "", err
	}
	token := cookie.Value
	jwtKey := os.Getenv("JWT_KEY")
	claims := &models.JwtCustomClaims{}
	_, err = jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtKey), nil
	})
	if err != nil {
		return "", err
	}
	return claims.Login, nil
}
