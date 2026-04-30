package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kenkaneki/expense-tracker/backend/handlers"
	"github.com/kenkaneki/expense-tracker/backend/middleware"
	"github.com/kenkaneki/expense-tracker/backend/store"
)

func main() {
	// Determine port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize SQLite store
	db, err := store.NewSQLiteStore("expenses.db")
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create handler
	expenseHandler := handlers.NewExpenseHandler(db)

	// Setup router
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("POST /api/expenses", expenseHandler.CreateExpense)
	mux.HandleFunc("GET /api/expenses", expenseHandler.ListExpenses)
	mux.HandleFunc("GET /api/expenses/summary", expenseHandler.GetSummary)

	// Serve frontend static files
	frontendDir := "../frontend"
	if envDir := os.Getenv("FRONTEND_DIR"); envDir != "" {
		frontendDir = envDir
	}
	mux.Handle("/", http.FileServer(http.Dir(frontendDir)))

	// Apply middleware
	handler := middleware.CORS(middleware.Logger(mux))

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Server starting on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
