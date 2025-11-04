package service

import (
    "context"
    "errors"
    "os"
    "strconv"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog/log"
)

type CreateTransferRequest struct {
    PalletID     string `json:"pallet_id"`
    FromLocation string `json:"from_location"`
    ToLocation   string `json:"to_location"`
    RequestedBy  string `json:"requested_by"`
}

type Transfer struct {
    ID           string    `json:"id"`
    PalletID     string    `json:"pallet_id"`
    FromLocation string    `json:"from_location"`
    ToLocation   string    `json:"to_location"`
    Status       string    `json:"status"`
    RequestedBy  string    `json:"requested_by"`
    ApprovedBy   *string   `json:"approved_by,omitempty"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

var (
    ErrNotFound         = errors.New("not found")
    ErrCapacityExceeded = errors.New("capacity exceeded")
)

type Repo interface {
    CreateTransfer(ctx context.Context, t *Transfer, idempotencyKey string) error
    GetTransfer(ctx context.Context, id string) (*Transfer, error)
    UpdateTransferStatus(ctx context.Context, id, status string, approvedBy *string) error
    CountByDestination(ctx context.Context, to string) (int, error)
    InsertOutbox(ctx context.Context, aggregateType, aggregateID, topic string, payload interface{}) error
    FlushOutboxAndMark(ctx context.Context, outboxDir string) error
    // Temperature methods are also in same repo implementation
    InsertReading(ctx context.Context, r TemperatureReading) error
    CreateAlert(ctx context.Context, a *Alert) error
    ListAlerts(ctx context.Context) ([]Alert, error)
}

type TransferService struct{
    repo Repo
    maxCapacity int
    validateCap bool
}

func NewTransferService(r Repo) *TransferService{
    max := 5
    if v := os.Getenv("MAX_CAPACITY_PER_LOCATION"); v != "" {
        if n, err := strconv.Atoi(v); err == nil { max = n }
    }
    validate := true
    if v := os.Getenv("VALIDATE_CAPACITY"); v != "" {
        validate = !(v == "false" || v == "0")
    }
    return &TransferService{repo: r, maxCapacity: max, validateCap: validate}
}

func (s *TransferService) CreateTransfer(ctx context.Context, req CreateTransferRequest, idempotencyKey string) (*Transfer, error) {
    if s.validateCap {
        count, err := s.repo.CountByDestination(ctx, req.ToLocation)
        if err != nil { return nil, err }
        if count >= s.maxCapacity { return nil, ErrCapacityExceeded }
    }
    id := uuid.New().String()
    now := time.Now().UTC()
    tr := &Transfer{ID:id, PalletID:req.PalletID, FromLocation:req.FromLocation, ToLocation:req.ToLocation, Status:"pending", RequestedBy:req.RequestedBy, CreatedAt:now, UpdatedAt:now}
    if err := s.repo.CreateTransfer(ctx, tr, idempotencyKey); err != nil {
        return nil, err
    }
    evt := map[string]interface{}{"transfer_id":tr.ID, "pallet_id":tr.PalletID, "from":tr.FromLocation, "to":tr.ToLocation, "status":tr.Status, "requested_by":tr.RequestedBy, "ts":tr.CreatedAt.Format(time.RFC3339)}
    if err := s.repo.InsertOutbox(ctx, "transfer", tr.ID, "transfer.created", evt); err != nil { return nil, err }
    log.Info().Str("event","transfer.created").Str("id",tr.ID).Msg("transfer created")
    return tr, nil
}

func (s *TransferService) AcceptTransfer(ctx context.Context, id string) error {
    _, err := s.repo.GetTransfer(ctx, id)
    if err != nil { return ErrNotFound }
    approved := "supervisor"
    if err := s.repo.UpdateTransferStatus(ctx, id, "accepted", &approved); err != nil { return err }
    evt := map[string]interface{}{"transfer_id":id, "approved_by":approved, "ts":time.Now().Format(time.RFC3339)}
    if err := s.repo.InsertOutbox(ctx, "transfer", id, "transfer.accepted", evt); err != nil { return err }
    log.Info().Str("event","transfer.accepted").Str("id",id).Msg("transfer accepted")
    return nil
}

func (s *TransferService) CompleteTransfer(ctx context.Context, id string) error {
    _, err := s.repo.GetTransfer(ctx, id)
    if err != nil { return ErrNotFound }
    if err := s.repo.UpdateTransferStatus(ctx, id, "completed", nil); err != nil { return err }
    evt := map[string]interface{}{"transfer_id":id, "processed_by":"operator","ts":time.Now().Format(time.RFC3339)}
    if err := s.repo.InsertOutbox(ctx, "transfer", id, "transfer.completed", evt); err != nil { return err }
    log.Info().Str("event","transfer.completed").Str("id",id).Msg("transfer completed")
    return nil
}

func (s *TransferService) GetTransfer(ctx context.Context, id string) (*Transfer, error) { return s.repo.GetTransfer(ctx, id) }
