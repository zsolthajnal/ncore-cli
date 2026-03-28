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

	// Collect aired episodes not yet on disk or downloaded
	var toDownload []Episode
	for _, ep := range episodes {
		if ep.Season == 0 || ep.Number == 0 {
			continue // skip specials
		}
		if !ep.HasAired() {
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
		if t := pickBest(results, key); t != nil {
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
// episode key (e.g. "S01E05").
func pickBest(results []Torrent, epKey string) *Torrent {
	keyLower := strings.ToLower(epKey)
	bestSeeds := -1
	var best *Torrent
	for i := range results {
		t := &results[i]
		if !strings.Contains(strings.ToLower(t.Name), keyLower) {
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
