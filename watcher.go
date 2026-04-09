package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// WatchConfig holds all runtime configuration for the watcher.
type WatchConfig struct {
	MediaDir    string
	TorrentDir  string
	StateFile   string
	Interval    time.Duration
	DownloadCmd string // template: {torrent} and {out_dir} are replaced
	Once        bool
	NcoreUser   string
	NcorePass   string
	Categories  []string
	Require     []string // torrent name must contain at least one of these
	Exclude     []string // torrent name must not contain any of these
}

func defaultWatchCategories() []string {
	return []string{"hdser_hun", "hdser", "xvidser_hun", "xvidser", "dvdser_hun", "dvdser"}
}

func runWatcher(cfg WatchConfig) error {
	if err := os.MkdirAll(cfg.TorrentDir, 0755); err != nil {
		return fmt.Errorf("create torrent dir: %w", err)
	}

	state, err := loadState(cfg.StateFile)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	client, err := newClient()
	if err != nil {
		return err
	}
	if err := client.login(cfg.NcoreUser, cfg.NcorePass); err != nil {
		return fmt.Errorf("ncore login: %w", err)
	}
	log.Println("Logged in to ncore.pro")

	for {
		log.Println("Starting watch cycle...")
		if err := runOnce(cfg, state, client); err != nil {
			log.Printf("Watch cycle error: %v", err)
		}
		if cfg.Once {
			return nil
		}
		log.Printf("Next check in %s", cfg.Interval)
		time.Sleep(cfg.Interval)
	}
}

func runOnce(cfg WatchConfig, state *State, client *Client) error {
	series, err := scanMediaDir(cfg.MediaDir)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	log.Printf("Found %d series directories", len(series))

	for _, s := range series {
		if err := processSeries(cfg, state, client, s); err != nil {
			log.Printf("[%s] %v", s.FolderName, err)
		}
		// Save state after each series so partial progress is not lost
		if err := state.save(); err != nil {
			log.Printf("State save error: %v", err)
		}
	}
	return nil
}

func processSeries(cfg WatchConfig, state *State, client *Client, scanned ScannedSeries) error {
	// Seed state with episodes already on disk
	for ep := range scanned.Episodes {
		state.markPresent(scanned.FolderName, ep)
	}

	ss := state.getSeries(scanned.FolderName)
	ss.FolderPath = scanned.FolderPath

	// Resolve TVMaze ID on first encounter
	if ss.TVMazeID == 0 {
		log.Printf("[%s] Looking up TVMaze: %q", scanned.FolderName, scanned.SearchName)
		show, err := tvmazeSearch(scanned.SearchName)
		if err != nil {
			return fmt.Errorf("tvmaze search: %w", err)
		}
		ss.TVMazeID = show.ID
		ss.Name = show.Name
		log.Printf("[%s] Matched → %q (TVMaze ID %d)", scanned.FolderName, show.Name, show.ID)
	}

	// Get the full episode list
	episodes, err := tvmazeEpisodes(ss.TVMazeID)
	if err != nil {
		return fmt.Errorf("tvmaze episodes: %w", err)
	}
	ss.LastChecked = time.Now()

	// Find the latest episode we know about (on disk or previously downloaded).
	// We only want episodes strictly after this — no backfilling gaps.
	latestSeason, latestEp := latestKnown(state, scanned.FolderName)

	// Collect aired episodes that come after what we already have.
	var toDownload []Episode
	for _, ep := range episodes {
		if ep.Season == 0 || ep.Number == 0 {
			continue // skip specials
		}
		if !ep.HasAired() {
			continue
		}
		// Skip anything at or before our latest known episode.
		if latestSeason > 0 && !episodeAfter(ep, latestSeason, latestEp) {
			continue
		}
		if state.isKnown(scanned.FolderName, ep.Key()) {
			continue
		}
		toDownload = append(toDownload, ep)
	}

	if len(toDownload) == 0 {
		log.Printf("[%s] Up to date (%d episodes on disk)", scanned.FolderName, len(scanned.Episodes))
		return nil
	}
	log.Printf("[%s] %d new episode(s) to download", scanned.FolderName, len(toDownload))

	for _, ep := range toDownload {
		if err := downloadEpisode(cfg, state, client, ss, ep); err != nil {
			log.Printf("[%s] %s: %v", scanned.FolderName, ep.Key(), err)
			state.markFailed(scanned.FolderName, ep.Key())
		}
	}
	return nil
}

func downloadEpisode(cfg WatchConfig, state *State, client *Client, ss *SeriesState, ep Episode) error {
	key := ep.Key()
	// Try series name from TVMaze first, then the folder search name
	query := fmt.Sprintf("%s %s", ss.Name, key)
	log.Printf("[%s] Searching ncore: %q", ss.FolderName, query)

	var best *Torrent
	for _, cat := range cfg.Categories {
		results, err := client.search(query, cat)
		if err != nil {
			log.Printf("[%s] search in %s: %v", ss.FolderName, cat, err)
			continue
		}
		if t := pickBest(results, key, cfg.Require, cfg.Exclude); t != nil {
			best = t
			break
		}
	}

	if best == nil {
		return fmt.Errorf("no ncore result found for %s", key)
	}
	log.Printf("[%s] %s: found %q (seeds: %s)", ss.FolderName, key, best.Name, best.Seeds)

	// Download .torrent file to staging dir
	torrentPath, err := client.downloadTorrentFile(best.ID, cfg.TorrentDir)
	if err != nil {
		return fmt.Errorf("save .torrent: %w", err)
	}

	// Run the download command (e.g. webtorrent-cli)
	outDir := ss.FolderPath
	log.Printf("[%s] %s: running download command → %s", ss.FolderName, key, outDir)
	if err := runDownloadCmd(cfg.DownloadCmd, torrentPath, outDir); err != nil {
		// Keep the .torrent file on failure so the user can inspect it
		return fmt.Errorf("download command: %w", err)
	}

	// Remove .torrent only on success
	_ = os.Remove(torrentPath)
	log.Printf("[%s] %s: done, .torrent removed", ss.FolderName, key)

	state.markDownloaded(ss.FolderName, key, best.ID)
	return nil
}

// pickBest returns the result with the most seeds whose name contains the
// episode key (e.g. "S01E05") and passes the require/exclude filters.
func pickBest(results []Torrent, epKey string, require, exclude []string) *Torrent {
	keyLower := strings.ToLower(epKey)
	bestSeeds := -1
	var best *Torrent
	for i := range results {
		t := &results[i]
		nameLower := strings.ToLower(t.Name)

		if !strings.Contains(nameLower, keyLower) {
			continue
		}
		// Must contain at least one of the require terms
		if len(require) > 0 {
			matched := false
			for _, r := range require {
				if strings.Contains(nameLower, strings.ToLower(r)) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		// Must not contain any of the exclude terms
		excluded := false
		for _, e := range exclude {
			if strings.Contains(nameLower, strings.ToLower(e)) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		seeds, _ := strconv.Atoi(t.Seeds)
		if seeds > bestSeeds {
			best = t
			bestSeeds = seeds
		}
	}
	return best
}

// latestKnown returns the highest (season, episode) pair that is marked
// present or downloaded for the given series. Returns (0,0) if nothing known.
func latestKnown(state *State, seriesName string) (int, int) {
	ss := state.getSeries(seriesName)
	latestS, latestE := 0, 0
	for key, ep := range ss.Episodes {
		if ep.Status != "present" && ep.Status != "downloaded" {
			continue
		}
		s, e := parseEpKey(key)
		if s > latestS || (s == latestS && e > latestE) {
			latestS, latestE = s, e
		}
	}
	return latestS, latestE
}

// episodeAfter returns true if ep comes after (latestS, latestE).
func episodeAfter(ep Episode, latestS, latestE int) bool {
	return ep.Season > latestS || (ep.Season == latestS && ep.Number > latestE)
}

// parseEpKey parses "S01E05" into (1, 5).
func parseEpKey(key string) (int, int) {
	var s, e int
	fmt.Sscanf(strings.ToUpper(key), "S%02dE%02d", &s, &e)
	return s, e
}

func runDownloadCmd(cmdTemplate, torrentPath, outDir string) error {
	cmd := strings.ReplaceAll(cmdTemplate, "{torrent}", torrentPath)
	cmd = strings.ReplaceAll(cmd, "{out_dir}", outDir)

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty download command")
	}
	c := exec.Command(parts[0], parts[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
