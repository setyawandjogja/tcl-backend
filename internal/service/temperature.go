package service

import (
    "context"
    "fmt"
    "os"
    "strconv"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog/log"
)

type TemperatureReading struct {
    RoomID string    `json:"room_id"`
    Temp   float64   `json:"temp"`
    Ts     time.Time `json:"ts"`
}

type Alert struct {
    ID      string    `json:"id"`
    RoomID  string    `json:"room_id"`
    Temp    float64   `json:"temp"`
    Level   string    `json:"level"`
    Message string    `json:"message"`
    Created time.Time `json:"created_at"`
}

type TemperatureService struct {
    repo Repo
    min float64
    max float64
}

func NewTemperatureService(r Repo) *TemperatureService {
    min := -5.0
    max := 8.0
    if v := os.Getenv("TEMP_MIN"); v != "" {
        if f, err := strconv.ParseFloat(v, 64); err == nil { min = f }
    }
    if v := os.Getenv("TEMP_MAX"); v != "" {
        if f, err := strconv.ParseFloat(v, 64); err == nil { max = f }
    }
    return &TemperatureService{repo: r, min: min, max: max}
}

func (s *TemperatureService) Ingest(ctx context.Context, readings []TemperatureReading) error {
    for _, rd := range readings {
        if rd.Ts.IsZero() {
            rd.Ts = time.Now().UTC()
        }
        if err := s.repo.InsertReading(ctx, rd); err != nil {
            return err
        }
        if rd.Temp < s.min || rd.Temp > s.max {
            a := &Alert{ID: uuid.New().String(), RoomID: rd.RoomID, Temp: rd.Temp, Level: "critical", Message: fmt.Sprintf("temp %.2f out of bounds (%.2f..%.2f)", rd.Temp, s.min, s.max), Created: time.Now().UTC()}
            if err := s.repo.CreateAlert(ctx, a); err != nil { return err }
            evt := map[string]interface{}{"room_id":a.RoomID, "temp":a.Temp, "level":a.Level, "message":a.Message, "ts":a.Created.Format(time.RFC3339)}
            if err := s.repo.InsertOutbox(ctx, "temperature", a.ID, "temperature.alert", evt); err != nil { return err }
            log.Info().Str("event","temperature.alert").Str("room",a.RoomID).Float64("temp",a.Temp).Msg("alert created")
        }
    }
    return nil
}

func (s *TemperatureService) ListAlerts(ctx context.Context) ([]Alert, error) {
    return s.repo.ListAlerts(ctx)
}
