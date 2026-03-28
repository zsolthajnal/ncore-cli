package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State is the persistent record of tracked series and downloaded episodes.
// It is serialised as JSON to a single file.
type State struct {
	mu     sync.Mutex
	path   string
	Series map[string]*SeriesState `json:"series"`
}

type SeriesState struct {
	TVMazeID    int                      `json:"tvmaze_id"`
	Name        string                   `json:"name"`         // canonical TVMaze name
	FolderName  string                   `json:"folder_name"`
	FolderPath  string                   `json:"folder_path"`
	LastChecked time.Time                `json:"last_checked"`
	Episodes    map[string]*EpisodeState `json:"episodes"` // key: "S01E01"
}

type EpisodeState struct {
	// "present"    – found on disk at scan time, not downloaded by us
	// "downloaded" – downloaded by this tool
	// "failed"     – last download attempt failed
	Status       string    `json:"status"`
	TorrentID    string    `json:"torrent_id,omitempty"`
	DownloadedAt time.Time `json:"downloaded_at,omitempty"`
}

func loadState(path string) (*State, error) {
	s := &State{
		path:   path,
		Series: make(map[string]*SeriesState),
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	// Restore path (not serialised)
	s.path = path
	return s, nil
}

func (s *State) save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

// getSeries returns or creates the SeriesState for a folder name.
// Caller must hold mu or be the only goroutine.
func (s *State) getSeries(folderName string) *SeriesState {
	if s.Series == nil {
		s.Series = make(map[string]*SeriesState)
	}
	if _, ok := s.Series[folderName]; !ok {
		s.Series[folderName] = &SeriesState{
			FolderName: folderName,
			Episodes:   make(map[string]*EpisodeState),
		}
	}
	return s.Series[folderName]
}

func (s *State) markPresent(folderName, epKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss := s.getSeries(folderName)
	if ss.Episodes == nil {
		ss.Episodes = make(map[string]*EpisodeState)
	}
	if _, exists := ss.Episodes[epKey]; !exists {
		ss.Episodes[epKey] = &EpisodeState{Status: "present"}
	}
}

func (s *State) isKnown(folderName, epKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss, ok := s.Series[folderName]
	if !ok {
		return false
	}
	ep, ok := ss.Episodes[epKey]
	if !ok {
		return false
	}
	return ep.Status == "present" || ep.Status == "downloaded"
}

func (s *State) markDownloaded(folderName, epKey, torrentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss := s.getSeries(folderName)
	if ss.Episodes == nil {
		ss.Episodes = make(map[string]*EpisodeState)
	}
	ss.Episodes[epKey] = &EpisodeState{
		Status:       "downloaded",
		TorrentID:    torrentID,
		DownloadedAt: time.Now(),
	}
}

func (s *State) markFailed(folderName, epKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss := s.getSeries(folderName)
	if ss.Episodes == nil {
		ss.Episodes = make(map[string]*EpisodeState)
	}
	ss.Episodes[epKey] = &EpisodeState{Status: "failed"}
}
