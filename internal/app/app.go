package app

import (
	"log/slog"
	grpcapp "sso/internal/app/grpc"
	"sso/internal/services/auth"
	"sso/internal/storage/postgres"
	"time"
)

type App struct {
	GRPCServer *grpcapp.App
	Storage    *postgres.Storage
}

func New(log *slog.Logger, grpcPort int, tokenTTL time.Duration) *App {
	storage, err := postgres.New()
	if err != nil {
		panic(err)
	}

	authService := auth.New(log, storage, storage, storage, storage, tokenTTL)

	grpcApp := grpcapp.New(log, authService, grpcPort)

	return &App{
		GRPCServer: grpcApp,
	}
}
