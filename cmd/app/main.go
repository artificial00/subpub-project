package main

import (
	"context"
	"google.golang.org/grpc/reflection"
	"net"
	"os"
	"os/signal"
	"subpub-project/config"
	"subpub-project/internal/api/server"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/joho/godotenv"
	"subpub-project/internal/proto"
	"subpub-project/pkg/subpub"
)

func main() {
	_ = godotenv.Load()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg := config.Load()

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen")
	}

	grpcServer := grpc.NewServer()
	engine := subpub.NewSubPub()
	proto.RegisterPubSubServer(grpcServer, server.New(engine))
	reflection.Register(grpcServer)

	go func() {
		log.Info().Msgf("gRPC server listening on :%s", cfg.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC server failed")
		}
	}()

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	log.Info().Msg("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := engine.Close(ctx); err != nil {
		log.Error().Err(err).Msg("error closing subpub")
	}
	grpcServer.GracefulStop()
}
