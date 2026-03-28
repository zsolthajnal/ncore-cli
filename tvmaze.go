package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const tvmazeBase = "https://api.tvmaze.com"

type Show struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Episode struct {
	Season  int    `json:"season"`
	Number  int    `json:"number"`
	Name    string `json:"name"`
	Airdate string `json:"airdate"` // "2013-08-25"
}

func (e Episode) Key() string {
	return fmt.Sprintf("S%02dE%02d", e.Season, e.Number)
}

func (e Episode) HasAired() bool {
	if e.Airdate == "" {
		return false
	}
	t, err := time.Parse("2006-01-02", e.Airdate)
	if err != nil {
		return false
	}
	// Give a day of grace to account for timezone differences
	return t.Before(time.Now().Add(24 * time.Hour))
}

func tvmazeSearch(name string) (*Show, error) {
	resp, err := http.Get(tvmazeBase + "/search/shows?q=" + url.QueryEscape(name))
	if err != nil {
		return nil, fmt.Errorf("tvmaze request: %w", err)
	}
	defer resp.Body.Close()

	var results []struct {
		Show Show `json:"show"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("tvmaze decode: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no shows found for %q", name)
	}
	return &results[0].Show, nil
}

func tvmazeEpisodes(showID int) ([]Episode, error) {
	resp, err := http.Get(fmt.Sprintf("%s/shows/%d/episodes", tvmazeBase, showID))
	if err != nil {
		return nil, fmt.Errorf("tvmaze request: %w", err)
	}
	defer resp.Body.Close()

	var eps []Episode
	if err := json.NewDecoder(resp.Body).Decode(&eps); err != nil {
		return nil, fmt.Errorf("tvmaze decode: %w", err)
	}
	return eps, nil
}
