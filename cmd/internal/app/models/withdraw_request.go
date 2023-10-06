package models

type WithDrawRequest struct {
	OrderNumber string  `json:"order"`
	Sum         float64 `json:"sum"`
}
