package models

type Order struct {
	Number        string  `json:"number"`
	Status        string  `json:"status"`
	Accrual       float64 `json:"accrual"`
	UploadedAt    string  `json:"uploaded_at"`
	LastChangedAt string  `json:"last_changed_at"`
}
