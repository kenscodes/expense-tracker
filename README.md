# Expense Tracker

A production-quality personal finance tool to record, review, and understand where your money goes. Built as a monorepo with a **Go backend** and **vanilla JavaScript frontend**.

🔗 **Live App**: _[Deployed on Render]_  
📂 **Repository**: _[GitHub]_

---

## Features

- **Add expenses** with amount, category, description, and date
- **View expenses** in a sortable, filterable table
- **Filter by category** and **sort by date** (newest/oldest first)
- **Real-time totals** that update with filters
- **Category summary** with visual breakdown (bar chart + percentages)
- **Idempotent submissions** — safe against double-clicks, retries, and page refreshes
- **Form validation** — prevents negative amounts, missing fields, invalid dates
- **Loading, error, and empty states** for graceful UX under unreliable conditions

---

## Architecture

```
expense-tracker/
├── backend/
│   ├── main.go                  # Server entry point, routing, graceful shutdown
│   ├── models/expense.go        # Data models, validation, money utilities
│   ├── store/sqlite.go          # SQLite persistence + idempotency storage
│   ├── handlers/expenses.go     # HTTP handlers (POST, GET, summary)
│   ├── handlers/expenses_test.go # Unit tests
│   └── middleware/middleware.go  # CORS + request logging
├── frontend/
│   ├── index.html               # Semantic HTML5 UI
│   ├── style.css                # Dark glassmorphism design system
│   └── app.js                   # API client, idempotency, state management
├── render.yaml                  # Render deployment config
└── README.md
```

---

## What Matters for Production (Design Justifications)

As per the assignment requirements, this project prioritizes **data correctness, resilience under network unreliability, and code clarity** over feature breadth. 

### 1. Robust Money Handling
Floating-point math in Javascript/Go can cause massive bugs (e.g. `0.1 + 0.2 = 0.30000000000000004`). To achieve true production-readiness, money is passed to the backend, immediately converted to **integer paise** (cents), and exclusively stored and calculated in the database as integers. It is only converted back to strings for display at the very edge of the API.

### 2. Idempotency & Realistic Network Conditions
The prompt specifically required the system to handle: *unreliable networks, browser refreshes, retries, and double clicks.* 
- Every form submission dynamically generates a `UUIDv4` idempotency key.
- The Go backend verifies this key inside an ACID database transaction.
- If a user loses connection and aggressively clicks the submit button 5 times, the UI handles the loading state natively, and if 5 identical requests somehow reach the backend, the database safely drops duplicates and returns a `200 OK` (instead of `201 Created`) without crashing.

### 3. Graceful UI States
Instead of assuming happy paths, the vanilla JS frontend explicitly manages `loading`, `error`, `empty`, and `data` states. If the server is unreachable, the user sees a specific error state with a "Retry" button rather than a silently broken page.

### 4. SQLite with WAL Mode
SQLite provides rock-solid ACID compliance with zero configuration burden for reviewing. However, to make it behave like a production database under load, `_journal_mode=WAL` (Write-Ahead Logging) is enabled upon initialization. This prevents write operations (adding an expense) from blocking read operations (listing expenses).

---

## Trade-offs Made Because of the Timebox

| Decision | Why? | How to upgrade for production |
|----------|-----------|------------------|
| **SQLite on ephemeral disk** | Allows reviewers to test locally without installing Docker or PostgreSQL. However, on free-tier platforms like Render, local disk gets wiped on redeploy. | Migrate to a managed PostgreSQL cluster (e.g., Supabase/RDS) by swapping out the `go-sqlite3` driver. The `store` interface makes this trivial. |
| **No Pagination** | Building robust cursor-based pagination both in the DB query and the frontend Javascript adds significant scope. Given the personal nature of the tool, returning all rows is acceptable for testing. | Add `cursor` and `limit` query parameters to the API, and a "Load More" button to the UI. |
| **No Automated Frontend testing** | Timebox focused on edge-case testing in the backend (where the business logic lives) and manually verifying the UI flows. | Add Cypress or Playwright to run end-to-end tests validating the DOM states and idempotency behavior. |

---

## What I Intentionally Did Not Do

- **Over-engineer with React/Next.js:** A simple CRUD tool doesn't require a virtual DOM or complex build pipelines. Vanilla HTML/CSS/JS demonstrates fundamental DOM manipulation and state management cleanly, keeping the repository incredibly small.
- **Use an ORM (e.g. GORM):** The schema is a single table and an idempotency table. Using direct SQL queries with standard library bindings avoids the "ORM tax" and makes performance profiling far easier.
- **Implement Edit/Delete:** The core assignment specified *recording* and *reviewing*. Skipping edit/delete allowed me to invest heavily into idempotency and money-handling edge cases instead.

---

## Setup & Run

### Prerequisites
- Go 1.22+
- CGO enabled (for SQLite: `export CGO_ENABLED=1`)

### Local Development

```bash
# Clone
git clone https://github.com/<your-username>/expense-tracker.git
cd expense-tracker

# Run backend (serves frontend too)
cd backend
go run main.go
```

Open **http://localhost:8080** in your browser.

### Run Tests

```bash
cd backend
go test -v ./...
```

---

## API Reference

### `POST /api/expenses`

Create a new expense. Supports idempotent retries via `Idempotency-Key` header.

**Request:**
```json
{
  "amount": 150.50,
  "category": "Food",
  "description": "Lunch at cafe",
  "date": "2025-01-15"
}
```

**Headers:** `Idempotency-Key: <uuid>` (recommended)

**Response:** `201 Created` (new) or `200 OK` (idempotent replay)

### `GET /api/expenses`

List expenses with optional filtering and sorting.

**Query Parameters:**
- `category` — filter by category (e.g., `?category=Food`)
- `sort` — `date_desc` (default) or `date_asc`

**Response:**
```json
{
  "expenses": [...],
  "total": 15050,
  "total_display": "₹150.50",
  "count": 3
}
```

### `GET /api/expenses/summary`

Get total expenses grouped by category.

---

## Deployment

The app is configured for **Render** via `render.yaml`. Push to GitHub and connect the repo in Render dashboard — it auto-deploys on push.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22, `net/http`, `go-sqlite3` |
| Frontend | HTML5, CSS3, Vanilla JavaScript |
| Database | SQLite (WAL mode) |
| Deployment | Render |
| Testing | Go `testing` + `httptest` |
