package models

type AccountBalance struct {
	CurrentBalance float64 `json:"current"`
	Withdrawn      float64 `json:"withdrawn"`
}
