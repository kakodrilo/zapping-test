package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"zapping-test-service/internal/adapter/handler"
	"zapping-test-service/internal/adapter/jwtadapter"
	"zapping-test-service/internal/adapter/media"
	"zapping-test-service/internal/adapter/repo"
	"zapping-test-service/internal/db"
	"zapping-test-service/internal/usecase/authuc"
	"zapping-test-service/internal/usecase/streamuc"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using system environment")
	}

	database, err := db.InitDB()
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer database.Close()
	log.Println("database connected")

	// Infrastructure
	tokenSvc := jwtadapter.NewService(os.Getenv("JWT_SECRET"))
	mediaClient := media.NewNginxClient(os.Getenv("MEDIA_SERVER_URL"))
	streamRepo := repo.NewStreamRepository(database)
	segmentRepo := repo.NewSegmentRepository(database)
	userRepo := repo.NewUserRepository(database)

	// Use cases
	streamSvc := streamuc.NewService(streamRepo, segmentRepo, mediaClient)
	authSvc := authuc.NewService(userRepo, tokenSvc)

	// Seed segment metadata to DB (once) if not present
	count, warnings, err := streamSvc.LoadAll(context.Background())
	if err != nil {
		log.Fatalf("failed to load streams: %v", err)
	}
	for _, w := range warnings {
		log.Printf("warning: %v", w)
	}
	log.Printf("%d stream(s) loaded", count)

	// HTTP controllers
	streamH := handler.NewStreamHandler(streamSvc)
	authH := handler.NewAuthHandler(authSvc)

	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return handler.AuthMiddleware(tokenSvc, h)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/register", authH.Register)
	mux.HandleFunc("POST /api/login", authH.Login)
	mux.HandleFunc("GET /api/refresh", auth(authH.Refresh))
	mux.HandleFunc("GET /api/streams", auth(streamH.List))
	mux.HandleFunc("GET /stream/{id}/playlist.m3u8", auth(streamH.Playlist))
	mux.HandleFunc("GET /stream/{id}/segments/{file}", auth(streamH.Segment))

	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "*"
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler.CORSMiddleware(corsOrigin, mux)))
}
