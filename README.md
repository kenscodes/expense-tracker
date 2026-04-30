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

## Key Design Decisions

### Money Handling — Integer Paise
Amounts are stored as **integer paise** (1 INR = 100 paise). For example, ₹150.50 is stored as `15050`. This eliminates floating-point precision errors that are critical in financial applications. The conversion happens at the API boundary — the frontend sends decimal values, the backend converts and stores as integers.

### Idempotency for Safe Retries
Each form submission generates a **UUID v4 idempotency key** sent via the `Idempotency-Key` HTTP header. The backend stores these keys in a dedicated table and checks them **within a database transaction** before inserting. This means:
- **Double-clicks** → same key → only one expense created
- **Network retries** → same key → returns existing record (HTTP 200 vs 201)
- **Page refresh after submit** → key is cleared on success, new key on new data

### SQLite with WAL Mode
Chosen for zero-config deployment and ACID guarantees. WAL (Write-Ahead Logging) mode enables concurrent reads during writes. Indexes on `category` and `date` columns ensure fast filtering and sorting.

### Frontend Architecture
Vanilla HTML/CSS/JS with no build step — intentional simplicity. The dark glassmorphism design uses CSS custom properties for a consistent design system. State management (loading, error, empty, data) is explicit and covers all realistic user scenarios.

---

## Trade-offs (Due to Timebox)

| Decision | Trade-off |
|----------|-----------|
| **SQLite** | Not horizontally scalable. For production at scale, PostgreSQL with a cloud-managed service would be appropriate. |
| **In-process storage** | On Render free tier, the filesystem is ephemeral — data resets on redeploy. A managed database would solve this. |
| **No authentication** | Single-user tool. Multi-user would need auth + user-scoped data. |
| **No pagination** | Acceptable for personal use (<1000 expenses). Would add cursor-based pagination for larger datasets. |
| **No edit/delete** | Focused on the core CRUD subset specified. Easy to add with similar patterns. |

---

## What I Intentionally Did Not Do

- **No ORM** — Direct SQL is more transparent for the small schema and avoids the "ORM tax" of debugging generated queries.
- **No SPA framework** — Vanilla JS avoids build complexity and keeps the frontend deployable as static files.
- **No WebSocket/SSE** — Polling on page load is sufficient for a single-user tool.
- **No rate limiting** — Would add for production (middleware-based, per-IP).

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
