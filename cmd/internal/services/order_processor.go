package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/RyanTrue/go-shop/cmd/internal/app/models"
	"github.com/RyanTrue/go-shop/cmd/internal/repository"
	"go.uber.org/zap"
	"net/http"
	"sync"
	"time"
)

const (
	checkInterval = 10 * time.Second
	numWorkers    = 5
)

type OrderProcessor interface {
	ProcessOrders(ctx context.Context) error
}

type orderProcessor struct {
	Repo             repository.Repository
	logger           *zap.SugaredLogger
	accrualSystemURL string
}

func NewOrderProcessor(repo repository.Repository, accrualSystemURL string, logger *zap.SugaredLogger) OrderProcessor {
	return &orderProcessor{
		Repo:             repo,
		accrualSystemURL: accrualSystemURL,
		logger:           logger,
	}
}

func (o *orderProcessor) ProcessOrders(ctx context.Context) error {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	orderChan := make(chan string, 100)

	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go o.worker(ctx, orderChan, &wg)
	}

	for {
		select {
		case <-ctx.Done():
			close(orderChan)
			wg.Wait()
			return ctx.Err()
		case <-ticker.C:
			if err := o.fetchAndQueueOrders(ctx, orderChan); err != nil {
				o.logger.Errorw("Error processing new orders", "error", err)
			}

			if err := o.fetchAndQueueStaleOrders(ctx, orderChan); err != nil {
				o.logger.Errorw("Error processing stale orders", "error", err)
			}
		}
	}
}

func (o *orderProcessor) fetchAndQueueStaleOrders(ctx context.Context, orderChan chan string) error {

	staleThreshold := time.Minute * 2 //can be edited to any time interval
	orders, err := o.Repo.GetStaleProcessingOrders(ctx, staleThreshold)
	if err != nil {
		return err
	}

	for _, order := range orders {
		orderChan <- order.Number
	}
	return nil
}

func (o *orderProcessor) fetchAndQueueOrders(ctx context.Context, orderChan chan string) error {
	orders, err := o.Repo.GetNewOrders(ctx)
	if err != nil {
		return err
	}

	for _, order := range orders {
		orderChan <- order.Number
	}
	return nil
}

func fetchAccrualForOrder(accrualURL string, orderID string) (*models.AccrualResponse, error) {
	url := fmt.Sprintf(accrualURL+"/api/orders/%s", orderID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 response: %d", resp.StatusCode)
	}

	var accrualResp models.AccrualResponse
	err = json.NewDecoder(resp.Body).Decode(&accrualResp)
	return &accrualResp, err
}

func (o *orderProcessor) worker(ctx context.Context, orderChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for orderID := range orderChan {
		var retryCount int
		for {
			accrualResp, err := fetchAccrualForOrder(o.accrualSystemURL, orderID)
			if err != nil {
				o.logger.Errorw("Error fetching accrual for order", "orderID", orderID, "error", err)
				break
			}

			if accrualResp.Status == "REGISTERED" || accrualResp.Status == "PROCESSING" {
				if retryCount < 3 { //can be edited to any number of retries
					retryCount++
					time.Sleep(5 * time.Second) //can be edited to any time interval
					continue
				} else {
					break
				}
			} else if accrualResp.Status == "INVALID" {
				err := o.Repo.SetOrderStatusInvalid(ctx, orderID)
				if err != nil {
					o.logger.Errorw("Error setting order status to INVALID", "orderID", orderID, "error", err)
				}
				break
			} else {
				err = o.Repo.UpdateOrderStatus(ctx, accrualResp.OrderNumber, accrualResp.Status, accrualResp.Accrual)
				if err != nil {
					o.logger.Errorw("Error updating order status", "orderID", orderID, "error", err)
				}
				break
			}
		}
	}
}
