package sync

import "time"

// SyncResult resultado de sincronização
type SyncResult struct {
	EpicsSynced   int           `json:"epics_synced"`
	StoriesSynced int           `json:"stories_synced"`
	TasksSynced   int           `json:"tasks_synced"`
	Duration      time.Duration `json:"duration"`
	Errors        []string      `json:"errors"`
}

// SyncState estado de sincronização
type SyncState struct {
	Conflicts []Conflict `json:"conflicts"`
}
