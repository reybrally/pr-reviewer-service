package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"pr-reviewer-service/internal/service"
	"syscall"
	"time"

	apphttp "pr-reviewer-service/internal/http"
	"pr-reviewer-service/internal/migrations"
	"pr-reviewer-service/internal/repository/postgres"
)

func main() {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://pr_user:pr_pass@localhost:55432/pr_service?sslmode=disable"
	}

	db, err := postgres.New(dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("failed to close db: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := migrations.Run(ctx, db.Conn()); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	teamRepo := postgres.NewTeamRepo(db.Conn())
	userRepo := postgres.NewUserRepo(db.Conn())
	prRepo := postgres.NewPullRequestRepo(db.Conn())

	teamService := service.NewTeamService(teamRepo, userRepo)
	userService := service.NewUserService(userRepo)
	prService := service.NewPRService(userRepo, prRepo)

	mux := http.NewServeMux()
	handler := apphttp.NewHandler(teamService, userService, prService)
	handler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("starting pr-reviewer-service on :%s\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	log.Println("server stopped")
}
