package models

import (
	"errors"
	"fmt"
	"time"
)

// Expense represents an expense entry.
// Amount is stored as integer paise (1 INR = 100 paise) to avoid
// floating-point precision issues with real money.
type Expense struct {
	ID          string `json:"id"`
	Amount      int64  `json:"amount"`       // in paise (e.g., ₹100.50 = 10050)
	AmountStr   string `json:"amount_display"` // formatted display string like "₹100.50"
	Category    string `json:"category"`
	Description string `json:"description"`
	Date        string `json:"date"`       // ISO 8601 date: YYYY-MM-DD
	CreatedAt   string `json:"created_at"` // ISO 8601 datetime
}

// CreateExpenseRequest is the expected JSON body for creating an expense.
type CreateExpenseRequest struct {
	Amount      float64 `json:"amount"`      // accept decimal from frontend
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Date        string  `json:"date"` // YYYY-MM-DD
}

// Validate checks the request for correctness.
func (r *CreateExpenseRequest) Validate() error {
	if r.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if r.Amount > 100_000_000 { // 10 crore limit
		return errors.New("amount exceeds maximum allowed value")
	}
	if r.Category == "" {
		return errors.New("category is required")
	}
	if len(r.Category) > 100 {
		return errors.New("category must be 100 characters or fewer")
	}
	if r.Description == "" {
		return errors.New("description is required")
	}
	if len(r.Description) > 500 {
		return errors.New("description must be 500 characters or fewer")
	}
	if r.Date == "" {
		return errors.New("date is required")
	}
	// Validate date format
	if _, err := time.Parse("2006-01-02", r.Date); err != nil {
		return errors.New("date must be in YYYY-MM-DD format")
	}
	return nil
}

// AmountToPaise converts a decimal amount to integer paise.
// This is done carefully to avoid floating-point rounding issues.
func AmountToPaise(amount float64) int64 {
	return int64(amount*100 + 0.5)
}

// PaiseToDisplay converts paise to a formatted display string.
func PaiseToDisplay(paise int64) string {
	rupees := paise / 100
	p := paise % 100
	return fmt.Sprintf("₹%d.%02d", rupees, p)
}

// ErrorResponse is returned for API errors.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ListExpensesResponse wraps the list response with total.
type ListExpensesResponse struct {
	Expenses []Expense `json:"expenses"`
	Total    int64     `json:"total"`     // total in paise
	TotalStr string    `json:"total_display"` // formatted total
	Count    int       `json:"count"`
}

// CategorySummary represents total expenses per category.
type CategorySummary struct {
	Category string `json:"category"`
	Total    int64  `json:"total"`
	TotalStr string `json:"total_display"`
	Count    int    `json:"count"`
}

// SummaryResponse wraps the summary data.
type SummaryResponse struct {
	Categories []CategorySummary `json:"categories"`
	GrandTotal int64             `json:"grand_total"`
	GrandStr   string            `json:"grand_total_display"`
}
