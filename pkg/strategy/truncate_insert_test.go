package strategy

import (
	"context"
	"testing"

	"github.com/bruin-data/ingestr/internal/config"
)

func TestTruncateInsertStrategy_Execute_PassesFullRefreshToRead(t *testing.T) {
	job, src, _ := minimalJob()
	job.Config.IncrementalStrategy = config.StrategyTruncateInsert
	job.Config.FullRefresh = true
	job.Config.PrimaryKeys = nil
	job.Schema.PrimaryKeys = nil
	src.readCh = mustClosedRecords()

	strat := &TruncateInsertStrategy{}
	if err := strat.Execute(context.Background(), job); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	src.mu.Lock()
	defer src.mu.Unlock()
	if !src.readOpts.FullRefresh {
		t.Fatalf("ReadOptions.FullRefresh = false, want true")
	}
}
