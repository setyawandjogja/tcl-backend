package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"transfer-service/internal/service"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type PostgresRepo struct {
	DB *sql.DB
}

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{DB: db} }

func AutoMigrate(db *sql.DB) error {
	createTransfers := `CREATE TABLE IF NOT EXISTS transfers (
        id TEXT PRIMARY KEY,
        pallet_id TEXT NOT NULL,
        from_location TEXT NOT NULL,
        to_location TEXT NOT NULL,
        status TEXT NOT NULL,
        requested_by TEXT NOT NULL,
        approved_by TEXT,
        idempotency_key TEXT UNIQUE,
        created_at TIMESTAMP DEFAULT NOW(),
        updated_at TIMESTAMP DEFAULT NOW()
    );`
	if _, err := db.Exec(createTransfers); err != nil {
		return err
	}

	createReadings := `CREATE TABLE IF NOT EXISTS temperature_readings (
        id TEXT PRIMARY KEY,
        room_id TEXT NOT NULL,
        temp DOUBLE PRECISION NOT NULL,
        recorded_at TIMESTAMP NOT NULL,
        created_at TIMESTAMP DEFAULT NOW()
    );`
	if _, err := db.Exec(createReadings); err != nil {
		return err
	}

	createAlerts := `CREATE TABLE IF NOT EXISTS alerts (
        id TEXT PRIMARY KEY,
        room_id TEXT NOT NULL,
        temp DOUBLE PRECISION NOT NULL,
        level TEXT NOT NULL,
        message TEXT,
        created_at TIMESTAMP DEFAULT NOW()
    );`
	if _, err := db.Exec(createAlerts); err != nil {
		return err
	}

	createOutbox := `CREATE TABLE IF NOT EXISTS outbox (
        id TEXT PRIMARY KEY,
        aggregate_type TEXT NOT NULL,
        aggregate_id TEXT NOT NULL,
        payload JSONB NOT NULL,
        published BOOLEAN DEFAULT FALSE,
        created_at TIMESTAMP DEFAULT NOW()
    );`
	if _, err := db.Exec(createOutbox); err != nil {
		return err
	}

	log.Info().Msg("migrations applied")
	return nil
}

// Transfer methods
func (r *PostgresRepo) CreateTransfer(ctx context.Context, t *service.Transfer, idempotencyKey string) error {
	q := `INSERT INTO transfers (id, pallet_id, from_location, to_location, status, requested_by, idempotency_key, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	_, err := r.DB.ExecContext(ctx, q, t.ID, t.PalletID, t.FromLocation, t.ToLocation, t.Status, t.RequestedBy, idempotencyKey, now, now)
	return err
}

func (r *PostgresRepo) GetTransfer(ctx context.Context, id string) (*service.Transfer, error) {
	q := `SELECT id,pallet_id,from_location,to_location,status,requested_by,approved_by,created_at,updated_at FROM transfers WHERE id=$1`
	row := r.DB.QueryRowContext(ctx, q, id)
	var t service.Transfer
	var approved sql.NullString
	if err := row.Scan(&t.ID, &t.PalletID, &t.FromLocation, &t.ToLocation, &t.Status, &t.RequestedBy, &approved, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	if approved.Valid {
		t.ApprovedBy = &approved.String
	}
	return &t, nil
}

func (r *PostgresRepo) UpdateTransferStatus(ctx context.Context, id, status string, approvedBy *string) error {
	q := `UPDATE transfers SET status=$1, approved_by=$2, updated_at=NOW() WHERE id=$3`
	_, err := r.DB.ExecContext(ctx, q, status, approvedBy, id)
	return err
}

func (r *PostgresRepo) CountByDestination(ctx context.Context, to string) (int, error) {
	q := `SELECT COUNT(1) FROM transfers WHERE to_location=$1 AND status IN ('pending','accepted','in_progress')`
	row := r.DB.QueryRowContext(ctx, q, to)
	var c int
	if err := row.Scan(&c); err != nil {
		return 0, err
	}
	return c, nil
}

// Outbox methods
func (r *PostgresRepo) InsertOutbox(ctx context.Context, aggregateType, aggregateID, topic string, payload interface{}) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	id := uuid.New().String()
	q := `INSERT INTO outbox (id, aggregate_type, aggregate_id, payload, published, created_at) VALUES ($1,$2,$3,$4,false,$5)`
	_, err = r.DB.ExecContext(ctx, q, id, aggregateType, aggregateID, string(b), time.Now().UTC())
	return err
}

type OutboxEvent struct {
	ID            string `json:"id"`
	AggregateType string `json:"aggregate_type"`
	AggregateID   string `json:"aggregate_id"`
	Payload       string `json:"payload"`
}

func (r *PostgresRepo) FlushOutboxAndMark(ctx context.Context, outboxDir string) error {
	q := `SELECT id, aggregate_type, aggregate_id, payload FROM outbox WHERE published = false ORDER BY created_at ASC`
	rows, err := r.DB.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	var events []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		if err := rows.Scan(&e.ID, &e.AggregateType, &e.AggregateID, &e.Payload); err != nil {
			return err
		}
		events = append(events, e)
	}
	for _, ev := range events {
		fname := fmt.Sprintf("%s_%s.json", ev.AggregateType, ev.ID)
		path := filepath.Join(outboxDir, fname)
		if err := ioutil.WriteFile(path, []byte(ev.Payload), 0o644); err != nil {
			return err
		}
		if _, err := r.DB.ExecContext(ctx, `UPDATE outbox SET published=true WHERE id=$1`, ev.ID); err != nil {
			return err
		}
		log.Info().Str("outbox_file", path).Msg("wrote outbox event")
	}
	return nil
}

// Temperature methods
func (r *PostgresRepo) InsertReading(ctx context.Context, rd service.TemperatureReading) error {
	q := `INSERT INTO temperature_readings (id, room_id, temp, recorded_at, created_at) VALUES ($1,$2,$3,$4,$5)`
	id := uuid.New().String()
	_, err := r.DB.ExecContext(ctx, q, id, rd.RoomID, rd.Temp, rd.Ts, time.Now().UTC())
	return err
}

func (r *PostgresRepo) CreateAlert(ctx context.Context, a *service.Alert) error {
	q := `INSERT INTO alerts (id, room_id, temp, level, message, created_at) VALUES ($1,$2,$3,$4,$5,$6)`
	_, err := r.DB.ExecContext(ctx, q, a.ID, a.RoomID, a.Temp, a.Level, a.Message, a.Created)
	return err
}

func (r *PostgresRepo) ListAlerts(ctx context.Context) ([]service.Alert, error) {
	q := `SELECT id, room_id, temp, level, message, created_at FROM alerts ORDER BY created_at DESC LIMIT 100`
	rows, err := r.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []service.Alert
	for rows.Next() {
		var a service.Alert
		if err := rows.Scan(&a.ID, &a.RoomID, &a.Temp, &a.Level, &a.Message, &a.Created); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, nil
}
