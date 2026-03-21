package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"habitual/internal/db"
	"habitual/internal/handler"
	"habitual/internal/service"
)

func main() {
	ctx := context.Background()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	habitSvc := service.NewHabitService(pool)
	h := handler.New(habitSvc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
