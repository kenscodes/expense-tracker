package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kenkaneki/expense-tracker/backend/models"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore manages expense persistence with SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-backed store and initializes the schema.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return store, nil
}

// migrate creates the required tables if they don't exist.
func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS expenses (
		id TEXT PRIMARY KEY,
		amount INTEGER NOT NULL,
		category TEXT NOT NULL,
		description TEXT NOT NULL,
		date TEXT NOT NULL,
		created_at TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_expenses_category ON expenses(category);
	CREATE INDEX IF NOT EXISTS idx_expenses_date ON expenses(date);

	CREATE TABLE IF NOT EXISTS idempotency_keys (
		key TEXT PRIMARY KEY,
		expense_id TEXT NOT NULL,
		created_at TEXT NOT NULL,
		FOREIGN KEY (expense_id) REFERENCES expenses(id)
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

// CreateExpense inserts a new expense with idempotency support.
// If the idempotencyKey was already used, it returns the existing expense.
// Returns (expense, created, error) where created indicates if a new record was made.
func (s *SQLiteStore) CreateExpense(expense models.Expense, idempotencyKey string) (models.Expense, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return models.Expense{}, false, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Check idempotency key first
	if idempotencyKey != "" {
		var existingID string
		err := tx.QueryRow(
			"SELECT expense_id FROM idempotency_keys WHERE key = ?",
			idempotencyKey,
		).Scan(&existingID)

		if err == nil {
			// Key exists — return the previously created expense
			existing, err := s.getExpenseByIDTx(tx, existingID)
			if err != nil {
				return models.Expense{}, false, fmt.Errorf("fetching existing expense: %w", err)
			}
			return existing, false, tx.Commit()
		}
		if err != sql.ErrNoRows {
			return models.Expense{}, false, fmt.Errorf("checking idempotency key: %w", err)
		}
	}

	// Insert the expense
	now := time.Now().UTC().Format(time.RFC3339)
	expense.CreatedAt = now
	expense.AmountStr = models.PaiseToDisplay(expense.Amount)

	_, err = tx.Exec(
		"INSERT INTO expenses (id, amount, category, description, date, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		expense.ID, expense.Amount, expense.Category, expense.Description, expense.Date, expense.CreatedAt,
	)
	if err != nil {
		return models.Expense{}, false, fmt.Errorf("inserting expense: %w", err)
	}

	// Store idempotency key
	if idempotencyKey != "" {
		_, err = tx.Exec(
			"INSERT INTO idempotency_keys (key, expense_id, created_at) VALUES (?, ?, ?)",
			idempotencyKey, expense.ID, now,
		)
		if err != nil {
			return models.Expense{}, false, fmt.Errorf("storing idempotency key: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return models.Expense{}, false, fmt.Errorf("committing transaction: %w", err)
	}

	return expense, true, nil
}

// getExpenseByIDTx fetches a single expense within a transaction.
func (s *SQLiteStore) getExpenseByIDTx(tx *sql.Tx, id string) (models.Expense, error) {
	var e models.Expense
	err := tx.QueryRow(
		"SELECT id, amount, category, description, date, created_at FROM expenses WHERE id = ?",
		id,
	).Scan(&e.ID, &e.Amount, &e.Category, &e.Description, &e.Date, &e.CreatedAt)
	if err != nil {
		return models.Expense{}, err
	}
	e.AmountStr = models.PaiseToDisplay(e.Amount)
	return e, nil
}

// ListExpenses returns expenses with optional filtering and sorting.
func (s *SQLiteStore) ListExpenses(category string, sort string) ([]models.Expense, error) {
	query := "SELECT id, amount, category, description, date, created_at FROM expenses"
	var args []interface{}
	var conditions []string

	if category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Default sort: newest first
	if sort == "date_desc" || sort == "" {
		query += " ORDER BY date DESC, created_at DESC"
	} else if sort == "date_asc" {
		query += " ORDER BY date ASC, created_at ASC"
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying expenses: %w", err)
	}
	defer rows.Close()

	var expenses []models.Expense
	for rows.Next() {
		var e models.Expense
		if err := rows.Scan(&e.ID, &e.Amount, &e.Category, &e.Description, &e.Date, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning expense: %w", err)
		}
		e.AmountStr = models.PaiseToDisplay(e.Amount)
		expenses = append(expenses, e)
	}

	return expenses, rows.Err()
}

// GetCategories returns a list of distinct categories.
func (s *SQLiteStore) GetCategories() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT category FROM expenses ORDER BY category")
	if err != nil {
		return nil, fmt.Errorf("querying categories: %w", err)
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

// GetSummary returns total expenses grouped by category.
func (s *SQLiteStore) GetSummary() ([]models.CategorySummary, error) {
	rows, err := s.db.Query(
		"SELECT category, SUM(amount) as total, COUNT(*) as count FROM expenses GROUP BY category ORDER BY total DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("querying summary: %w", err)
	}
	defer rows.Close()

	var summaries []models.CategorySummary
	for rows.Next() {
		var cs models.CategorySummary
		if err := rows.Scan(&cs.Category, &cs.Total, &cs.Count); err != nil {
			return nil, fmt.Errorf("scanning summary: %w", err)
		}
		cs.TotalStr = models.PaiseToDisplay(cs.Total)
		summaries = append(summaries, cs)
	}
	return summaries, rows.Err()
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
