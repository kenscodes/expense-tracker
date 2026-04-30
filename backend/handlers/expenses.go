package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/kenkaneki/expense-tracker/backend/models"
	"github.com/kenkaneki/expense-tracker/backend/store"
)

// ExpenseHandler handles HTTP requests for expenses.
type ExpenseHandler struct {
	store *store.SQLiteStore
}

// NewExpenseHandler creates a new ExpenseHandler.
func NewExpenseHandler(store *store.SQLiteStore) *ExpenseHandler {
	return &ExpenseHandler{store: store}
}

// CreateExpense handles POST /api/expenses.
// Supports idempotency via the Idempotency-Key header to safely handle
// retries, double-clicks, and page refreshes.
func (h *ExpenseHandler) CreateExpense(w http.ResponseWriter, r *http.Request) {
	var req models.CreateExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid JSON body",
		})
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	// Extract idempotency key
	idempotencyKey := r.Header.Get("Idempotency-Key")

	// Convert amount to paise
	amountPaise := models.AmountToPaise(req.Amount)

	expense := models.Expense{
		ID:          uuid.New().String(),
		Amount:      amountPaise,
		Category:    req.Category,
		Description: req.Description,
		Date:        req.Date,
	}

	result, created, err := h.store.CreateExpense(expense, idempotencyKey)
	if err != nil {
		log.Printf("error creating expense: %v", err)
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to create expense",
		})
		return
	}

	status := http.StatusCreated
	if !created {
		// Idempotent replay — return 200 instead of 201
		status = http.StatusOK
	}

	writeJSON(w, status, result)
}

// ListExpenses handles GET /api/expenses.
// Supports query parameters:
//   - category: filter by category
//   - sort: "date_desc" (default) or "date_asc"
func (h *ExpenseHandler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	sort := r.URL.Query().Get("sort")

	// Validate sort parameter
	if sort != "" && sort != "date_desc" && sort != "date_asc" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "sort must be 'date_desc' or 'date_asc'",
		})
		return
	}

	expenses, err := h.store.ListExpenses(category, sort)
	if err != nil {
		log.Printf("error listing expenses: %v", err)
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to list expenses",
		})
		return
	}

	// Ensure we return an empty array, not null
	if expenses == nil {
		expenses = []models.Expense{}
	}

	// Calculate total
	var total int64
	for _, e := range expenses {
		total += e.Amount
	}

	response := models.ListExpensesResponse{
		Expenses: expenses,
		Total:    total,
		TotalStr: models.PaiseToDisplay(total),
		Count:    len(expenses),
	}

	writeJSON(w, http.StatusOK, response)
}

// GetSummary handles GET /api/expenses/summary.
// Returns total expenses grouped by category.
func (h *ExpenseHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	summaries, err := h.store.GetSummary()
	if err != nil {
		log.Printf("error getting summary: %v", err)
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to get summary",
		})
		return
	}

	if summaries == nil {
		summaries = []models.CategorySummary{}
	}

	var grandTotal int64
	for _, s := range summaries {
		grandTotal += s.Total
	}

	response := models.SummaryResponse{
		Categories: summaries,
		GrandTotal: grandTotal,
		GrandStr:   models.PaiseToDisplay(grandTotal),
	}

	writeJSON(w, http.StatusOK, response)
}

// writeJSON encodes the response as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("error encoding JSON response: %v", err)
	}
}
