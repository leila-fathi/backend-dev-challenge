package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hiring-challenge-backend/api/graph/generated"
	"hiring-challenge-backend/api/graph/resolver"
	"hiring-challenge-backend/api/internal/auth"
	"hiring-challenge-backend/api/internal/config"
	"hiring-challenge-backend/api/internal/db"
	"hiring-challenge-backend/api/internal/hasura"
	"hiring-challenge-backend/api/internal/publicapi"
	"hiring-challenge-backend/api/internal/session"
	"hiring-challenge-backend/api/internal/upload"
	"hiring-challenge-backend/api/internal/zitadel"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	chicors "github.com/go-chi/cors"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	dbPool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer dbPool.Close()

	repo := publicapi.NewRepository(dbPool)
	sessionStore := session.NewStore(dbPool, cfg.SessionTTL)

	zitadelAuth, err := zitadel.NewAuthenticator(
		cfg.ZitadelDomain,
		cfg.ZitadelBrowserClientID,
		cfg.ZitadelServiceAccountKeyPath,
	)
	if err != nil {
		log.Fatalf("init zitadel authenticator: %v", err)
	}
	apiHandler := publicapi.NewHandler(repo, sessionStore, zitadelAuth)

	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("prepare upload directory: %v", err)
	}

	tokenManager := auth.NewManager(cfg.JWTSigningKey, cfg.JWTClaimsNamespace, cfg.AccessTokenTTL)
	hasuraClient := hasura.NewClient(cfg.HasuraGraphQLURL, cfg.HasuraAdminSecret)
	uploadHandler := upload.NewHandler(hasuraClient, cfg.UploadDir, cfg.ThumbnailMaxSize)

	gqlResolver := &resolver.Resolver{
		TokenManager: tokenManager,
		HasuraClient: hasuraClient,
	}

	gqlServer := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{
		Resolvers: gqlResolver,
	}))

	router := chi.NewRouter()
	router.Use(chimw.RealIP)
	router.Use(chimw.RequestID)
	router.Use(chimw.Recoverer)
	router.Use(chimw.Timeout(30 * time.Second))
	router.Use(chicors.Handler(chicors.Options{
		AllowedOrigins:   cfg.CORSAllowOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	router.Group(func(legacy chi.Router) {
		legacy.Use(auth.ContextMiddleware(tokenManager))
		legacy.Handle("/graphql", gqlServer)
		legacy.Post("/upload/image", uploadHandler.HandleUploadImage)
	})

	router.Get("/playground", playground.Handler("Challenge GraphQL", "/graphql"))

	router.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", apiHandler.Login)

		r.Group(func(pr chi.Router) {
			pr.Use(publicapi.AuthMiddleware(sessionStore))
			pr.Get("/auth/memberships", apiHandler.GetMemberships)
			pr.Post("/auth/switch-group", apiHandler.SwitchGroup)
			pr.Get("/projects", apiHandler.ListProjects)
			pr.Post("/projects", apiHandler.CreateProject)
		})
	})

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           router,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("API server listening on :%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown failed: %v", err)
	}
}

