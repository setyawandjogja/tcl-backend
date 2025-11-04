package main

import (
    "context"
    "database/sql"
    "fmt"
    "net/http"
    "os"
    "path/filepath"
    "time"

    _ "github.com/lib/pq"
    "github.com/go-chi/chi/v5"
    "github.com/joho/godotenv"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
    httpSwagger "github.com/swaggo/http-swagger"
    "github.com/prometheus/client_golang/prometheus/promhttp"

    "transfer-service/internal/handler"
    "transfer-service/internal/repo"
    "transfer-service/internal/service"
)

// @title Transfer & Temperature Service API
// @version 1.0
// @description API gabungan untuk manajemen transfer pallet dan pemantauan temperatur
// @host localhost:8080
// @BasePath /api

func main() {
    _ = godotenv.Load()

    zerolog.TimeFieldFormat = time.RFC3339
    logLevel := zerolog.InfoLevel
    if os.Getenv("LOG_LEVEL") == "debug" {
        logLevel = zerolog.DebugLevel
    }

    consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
    os.MkdirAll("logs", 0o755)
    f, err := os.OpenFile(filepath.Join("logs", "app.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
    if err != nil {
        log.Fatal().Err(err).Msg("open log file")
    }
    multi := zerolog.MultiLevelWriter(consoleWriter, f)
    log.Logger = log.Output(multi).Level(logLevel)
    defer f.Close()

    dbHost := os.Getenv("DB_HOST")
    dbPort := os.Getenv("DB_PORT")
    dbUser := os.Getenv("DB_USER")
    dbPass := os.Getenv("DB_PASS")
    dbName := os.Getenv("DB_NAME")
    if dbHost == "" { dbHost = "localhost" }
    if dbPort == "" { dbPort = "5432" }
    dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPass, dbName)

    db, err := sql.Open("postgres", dsn)
    if err != nil {
        log.Fatal().Err(err).Msg("connect db")
    }
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := db.PingContext(ctx); err != nil {
        log.Fatal().Err(err).Msg("ping db")
    }

    if err := repo.AutoMigrate(db); err != nil {
        log.Fatal().Err(err).Msg("migrate")
    }

    rep := repo.NewPostgresRepo(db)
    transferSvc := service.NewTransferService(rep)
    tempSvc := service.NewTemperatureService(rep)
    combined := service.NewCombinedService(transferSvc, tempSvc, rep)

    r := chi.NewRouter()
    r.Use(handler.ZeroLogRequestMiddleware)

    r.Mount("/api", handler.Routes(combined))
    r.Handle("/metrics", promhttp.Handler())
    r.Get("/swagger/*", httpSwagger.Handler(
        httpSwagger.URL("http://localhost:8080/swagger/doc.json"),
    ))

    addr := ":8080"
    log.Info().Msgf("starting gateway %s", addr)
    if err := http.ListenAndServe(addr, r); err != nil {
        log.Fatal().Err(err).Msg("server stopped")
    }
}
