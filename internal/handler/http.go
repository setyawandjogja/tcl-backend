package handler

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/rs/zerolog/log"
    "transfer-service/internal/service"
)

// Routes mounts all routes for transfer+temperature under /api
// @tags Transfers, Temperature, Dev, Monitoring
func Routes(svc *service.CombinedService) http.Handler {
    r := chi.NewRouter()

    // Transfer
    r.Post("/transfers", createTransferHandler(svc))
    r.Post("/transfers/{id}/accept", acceptTransferHandler(svc))
    r.Post("/transfers/{id}/complete", completeTransferHandler(svc))
    r.Get("/transfers/{id}", getTransferHandler(svc))
    r.Post("/dev/flush-outbox", flushOutboxHandler(svc))

    // Temperature
    r.Post("/temperatures", ingestTempHandler(svc))
    r.Get("/alerts", getAlertsHandler(svc))
    r.Post("/temperatures/dev/flush-outbox", flushOutboxHandler(svc))

    return r
}

// CreateTransfer godoc
// @Summary Membuat permintaan transfer pallet baru
// @Description Membuat transfer baru dengan validasi kapasitas lokasi tujuan.
// @Tags Transfers
// @Accept json
// @Produce json
// @Param Idempotency-Key header string false "Idempotency Key"
// @Param request body service.CreateTransferRequest true "Transfer Request Body"
// @Success 201 {object} service.Transfer
// @Failure 400 {object} map[string]string
// @Router /transfers [post]
func createTransferHandler(svc *service.CombinedService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req service.CreateTransferRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            log.Error().Err(err).Msg("invalid body")
            http.Error(w, "invalid body", http.StatusBadRequest)
            return
        }
        idempo := r.Header.Get("Idempotency-Key")
        tr, err := svc.Transfer.CreateTransfer(r.Context(), req, idempo)
        if err != nil {
            log.Error().Err(err).Msg("create transfer")
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(tr)
    }
}

// AcceptTransfer godoc
// @Summary Menerima transfer pallet
// @Tags Transfers
// @Param id path string true "Transfer ID"
// @Success 200
// @Router /transfers/{id}/accept [post]
func acceptTransferHandler(svc *service.CombinedService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := chi.URLParam(r, "id")
        if err := svc.Transfer.AcceptTransfer(r.Context(), id); err != nil {
            log.Error().Err(err).Msg("accept transfer")
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
    }
}

// CompleteTransfer godoc
// @Summary Menyelesaikan transfer pallet
// @Tags Transfers
// @Param id path string true "Transfer ID"
// @Success 200
// @Router /transfers/{id}/complete [post]
func completeTransferHandler(svc *service.CombinedService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := chi.URLParam(r, "id")
        if err := svc.Transfer.CompleteTransfer(r.Context(), id); err != nil {
            log.Error().Err(err).Msg("complete transfer")
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
    }
}

// GetTransfer godoc
// @Summary Mendapatkan detail transfer berdasarkan ID
// @Tags Transfers
// @Param id path string true "Transfer ID"
// @Produce json
// @Success 200 {object} service.Transfer
// @Failure 404 {object} map[string]string
// @Router /transfers/{id} [get]
func getTransferHandler(svc *service.CombinedService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := chi.URLParam(r, "id")
        tr, err := svc.Transfer.GetTransfer(r.Context(), id)
        if err != nil {
            log.Error().Err(err).Msg("get transfer")
            http.Error(w, err.Error(), http.StatusNotFound)
            return
        }
        json.NewEncoder(w).Encode(tr)
    }
}

// Ingest temperatures
// @Summary Ingest temperatures
// @Tags Temperature
// @Accept json
// @Param body body []service.TemperatureReading true "readings"
// @Success 200
// @Router /temperatures [post]
func ingestTempHandler(svc *service.CombinedService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var readings []service.TemperatureReading
        if err := json.NewDecoder(r.Body).Decode(&readings); err != nil {
            log.Error().Err(err).Msg("invalid body")
            http.Error(w, "invalid body", http.StatusBadRequest)
            return
        }
        if err := svc.Temperature.Ingest(r.Context(), readings); err != nil {
            log.Error().Err(err).Msg("ingest temp")
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
    }
}

// Get alerts
// @Summary Get alerts
// @Tags Temperature
// @Produce json
// @Success 200 {array} service.Alert
// @Router /alerts [get]
func getAlertsHandler(svc *service.CombinedService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        alerts, err := svc.Temperature.ListAlerts(r.Context())
        if err != nil {
            log.Error().Err(err).Msg("list alerts")
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        json.NewEncoder(w).Encode(alerts)
    }
}

func flushOutboxHandler(svc *service.CombinedService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if err := svc.FlushOutbox(r.Context()); err != nil {
            log.Error().Err(err).Msg("flush outbox")
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"status":"flushed"})
    }
}

// ZeroLogRequestMiddleware logs requests in JSON using zerolog
func ZeroLogRequestMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Info().Str("method", r.Method).Str("path", r.URL.Path).Dur("latency", time.Since(start)).Msg("request")
    })
}
