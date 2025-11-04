package service

import (
    "context"
    "os"
)

type CombinedService struct {
    Transfer    *TransferService
    Temperature *TemperatureService
    repo        Repo
}

func NewCombinedService(t *TransferService, temp *TemperatureService, r Repo) *CombinedService {
    return &CombinedService{Transfer: t, Temperature: temp, repo: r}
}

func (s *CombinedService) FlushOutbox(ctx context.Context) error {
    outdir := "outbox_events"
    if err := os.MkdirAll(outdir, 0o755); err != nil { return err }
    return s.repo.FlushOutboxAndMark(ctx, outdir)
}
