// main.go
package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"weibaobe/config"
	"weibaobe/database"
	"weibaobe/handlers"
	"weibaobe/storage"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Connect to PostgreSQL
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create tables
	if err := database.CreateUsersTable(db); err != nil {
		log.Fatalf("Failed to create users table: %v", err)
	}

	// Connect to Cloudflare R2
	r2Client, err := storage.NewR2Client(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to R2: %v", err)
	}

	// Initialize handlers
	h := &handlers.Handlers{
		DB:       db,
		R2Client: r2Client,
	}

	// Setup routes
	router := mux.NewRouter()
	router.HandleFunc("/users", h.CreateUser).Methods("POST")
	router.HandleFunc("/users", h.GetUsers).Methods("GET")
	router.HandleFunc("/upload", h.UploadFile).Methods("POST")

	// Start server
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
