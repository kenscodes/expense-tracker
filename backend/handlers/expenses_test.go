package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/kenkaneki/expense-tracker/backend/handlers"
	"github.com/kenkaneki/expense-tracker/backend/models"
	"github.com/kenkaneki/expense-tracker/backend/store"
)

func setupTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() {
		s.Close()
		os.Remove(dbPath)
	})
	return s
}

func TestCreateExpense_Success(t *testing.T) {
	s := setupTestStore(t)
	h := handlers.NewExpenseHandler(s)

	body := `{"amount": 150.50, "category": "Food", "description": "Lunch", "date": "2025-01-15"}`
	req := httptest.NewRequest(http.MethodPost, "/api/expenses", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "test-key-1")

	rr := httptest.NewRecorder()
	h.CreateExpense(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}

	var expense models.Expense
	if err := json.NewDecoder(rr.Body).Decode(&expense); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if expense.Amount != 15050 {
		t.Errorf("expected amount 15050 paise, got %d", expense.Amount)
	}
	if expense.Category != "Food" {
		t.Errorf("expected category Food, got %s", expense.Category)
	}
}

func TestCreateExpense_Idempotency(t *testing.T) {
	s := setupTestStore(t)
	h := handlers.NewExpenseHandler(s)

	body := `{"amount": 100, "category": "Transport", "description": "Cab", "date": "2025-01-15"}`
	idempotencyKey := "idempotent-key-123"

	// First request — should create
	req1 := httptest.NewRequest(http.MethodPost, "/api/expenses", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idempotencyKey)
	rr1 := httptest.NewRecorder()
	h.CreateExpense(rr1, req1)

	if rr1.Code != http.StatusCreated {
		t.Errorf("first request: expected 201, got %d", rr1.Code)
	}

	var expense1 models.Expense
	json.NewDecoder(rr1.Body).Decode(&expense1)

	// Second request with same key — should return existing, not create
	req2 := httptest.NewRequest(http.MethodPost, "/api/expenses", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", idempotencyKey)
	rr2 := httptest.NewRecorder()
	h.CreateExpense(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("second request: expected 200 (idempotent replay), got %d", rr2.Code)
	}

	var expense2 models.Expense
	json.NewDecoder(rr2.Body).Decode(&expense2)

	if expense1.ID != expense2.ID {
		t.Errorf("idempotent replay returned different ID: %s vs %s", expense1.ID, expense2.ID)
	}

	// Verify only one expense exists
	expenses, err := s.ListExpenses("", "")
	if err != nil {
		t.Fatalf("failed to list expenses: %v", err)
	}
	if len(expenses) != 1 {
		t.Errorf("expected 1 expense, got %d (idempotency failed)", len(expenses))
	}
}

func TestCreateExpense_Validation(t *testing.T) {
	s := setupTestStore(t)
	h := handlers.NewExpenseHandler(s)

	tests := []struct {
		name string
		body string
	}{
		{"negative amount", `{"amount": -50, "category": "Food", "description": "Test", "date": "2025-01-15"}`},
		{"zero amount", `{"amount": 0, "category": "Food", "description": "Test", "date": "2025-01-15"}`},
		{"missing category", `{"amount": 100, "category": "", "description": "Test", "date": "2025-01-15"}`},
		{"missing description", `{"amount": 100, "category": "Food", "description": "", "date": "2025-01-15"}`},
		{"missing date", `{"amount": 100, "category": "Food", "description": "Test", "date": ""}`},
		{"invalid date format", `{"amount": 100, "category": "Food", "description": "Test", "date": "15/01/2025"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/expenses", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.CreateExpense(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for %s, got %d", tc.name, rr.Code)
			}
		})
	}
}

func TestListExpenses_FilterAndSort(t *testing.T) {
	s := setupTestStore(t)
	h := handlers.NewExpenseHandler(s)

	// Create test expenses
	expenses := []string{
		`{"amount": 100, "category": "Food", "description": "Lunch", "date": "2025-01-10"}`,
		`{"amount": 200, "category": "Transport", "description": "Cab", "date": "2025-01-15"}`,
		`{"amount": 50, "category": "Food", "description": "Coffee", "date": "2025-01-12"}`,
	}

	for i, body := range expenses {
		req := httptest.NewRequest(http.MethodPost, "/api/expenses", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "setup-"+string(rune('0'+i)))
		rr := httptest.NewRecorder()
		h.CreateExpense(rr, req)
	}

	// Test: list all (newest first by default)
	t.Run("list all", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/expenses", nil)
		rr := httptest.NewRecorder()
		h.ListExpenses(rr, req)

		var resp models.ListExpensesResponse
		json.NewDecoder(rr.Body).Decode(&resp)

		if resp.Count != 3 {
			t.Errorf("expected 3 expenses, got %d", resp.Count)
		}
		// Check newest first
		if resp.Expenses[0].Date != "2025-01-15" {
			t.Errorf("expected newest first, got date %s", resp.Expenses[0].Date)
		}
	})

	// Test: filter by category
	t.Run("filter by Food", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/expenses?category=Food", nil)
		rr := httptest.NewRecorder()
		h.ListExpenses(rr, req)

		var resp models.ListExpensesResponse
		json.NewDecoder(rr.Body).Decode(&resp)

		if resp.Count != 2 {
			t.Errorf("expected 2 Food expenses, got %d", resp.Count)
		}
		// Verify total: 100 + 50 = 150 INR = 15000 paise
		if resp.Total != 15000 {
			t.Errorf("expected total 15000 paise, got %d", resp.Total)
		}
	})

	// Test: sort oldest first
	t.Run("sort oldest first", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/expenses?sort=date_asc", nil)
		rr := httptest.NewRecorder()
		h.ListExpenses(rr, req)

		var resp models.ListExpensesResponse
		json.NewDecoder(rr.Body).Decode(&resp)

		if resp.Expenses[0].Date != "2025-01-10" {
			t.Errorf("expected oldest first, got date %s", resp.Expenses[0].Date)
		}
	})
}
