package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"
)

func usage() {
	fmt.Fprint(os.Stderr, `ncore-cli — search, download and auto-watch series from ncore.pro

Environment variables:
  NCORE_USER   ncore.pro username (required for search/download/watch)
  NCORE_PASS   ncore.pro password (required for search/download/watch)

Commands (short aliases in parentheses):
  search (s) <query> [category]
      Search torrents. Category defaults to "all_own".
      Common categories: all_own, xvid_hun, xvid, dvd_hun, dvd,
        dvd9_hun, dvd9, hd_hun, hd, hdser_hun, hdser, mp3_hun, mp3,
        lossless_hun, lossless, ebook_hun, ebook, game_iso, game_rip,
        console, xxx_xvid, xxx_dvd, xxx_hd, xxx_imageset, misc, mobil

  download (d) <id> [output_dir]
      Download the torrent file for the given ID.
      Output directory defaults to the current working directory.

  scan (ls) <media_dir>
      Scan a media directory and show detected series + episode counts.

  watch (w) [flags]
      Scan media_dir, check TVMaze for new episodes, download via
      webtorrent-cli, and repeat on an interval.
      Flags:
        --media-dir      Directory to watch (required)
        --torrent-dir    Staging dir for .torrent files
                         (default: <media-dir>/torrent)
        --state          Path to state JSON file
                         (default: /var/lib/ncore-cli/state.json)
        --interval       Check interval, e.g. 6h, 30m (default: 6h)
        --download-cmd   Command template; use {torrent} and {out_dir}
                         (default: webtorrent download {torrent} --out {out_dir})
        --once           Run one cycle then exit (useful for cron)
        --require        Comma-separated terms the torrent name must contain
                         (e.g. "1080p")
        --exclude        Comma-separated terms the torrent name must not contain
                         (e.g. "2160p,2160,UHD,4K")

  install (i) [flags]
      Install ncore-cli as a systemd service (run as root).
      Accepts the same flags as watch.

  uninstall
      Stop and remove the ncore-cli systemd service (run as root).
      Credentials and state file are kept; remove manually if unwanted.

  completion <bash|zsh|fish>
      Output a shell completion script. Load with:
        bash : source <(ncore-cli completion bash)
        zsh  : source <(ncore-cli completion zsh)
        fish : ncore-cli completion fish > ~/.config/fish/completions/ncore-cli.fish

Examples:
  ncore-cli search "breaking bad" hdser_hun
  ncore-cli download 3995642 ~/Downloads
  ncore-cli scan /srv/data1/plexmediaserver/series
  ncore-cli watch --media-dir /srv/data1/plexmediaserver/series
  sudo ncore-cli install --media-dir /srv/data1/plexmediaserver/series
`)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "error: environment variable %s is not set\n", key)
		os.Exit(1)
	}
	return v
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

var version = "dev"

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		usage()
		if len(os.Args) < 2 {
			os.Exit(1)
		}
		return
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println(version)
	case "search", "s":
		cmdSearch(os.Args[2:])
	case "download", "d":
		cmdDownload(os.Args[2:])
	case "scan", "ls":
		cmdScan(os.Args[2:])
	case "watch", "w":
		cmdWatch(os.Args[2:], false)
	case "install", "i":
		cmdWatch(os.Args[2:], true)
	case "uninstall":
		if err := runUninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "completion":
		runCompletion(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func cmdSearch(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "error: search requires a query argument")
		os.Exit(1)
	}
	query := args[0]
	category := "all_own"
	if len(args) >= 2 {
		category = args[1]
	}

	client := mustLogin()
	results, err := client.search(query, category)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}

	fmt.Printf("\nFound %d result(s) for %q:\n\n", len(results), query)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSIZE\tSEEDS\tLEECHES\tTYPE")
	fmt.Fprintln(w, strings.Repeat("─", 8)+"\t"+strings.Repeat("─", 55)+"\t"+strings.Repeat("─", 10)+"\t"+strings.Repeat("─", 5)+"\t"+strings.Repeat("─", 7)+"\t"+strings.Repeat("─", 12))
	for _, t := range results {
		name := t.Name
		if len(name) > 55 {
			name = name[:52] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", t.ID, name, t.Size, t.Seeds, t.Leeches, t.Type)
	}
	w.Flush()
	fmt.Println()
}

func cmdDownload(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "error: download requires an ID argument")
		os.Exit(1)
	}
	id := args[0]
	outDir := "."
	if len(args) >= 2 {
		outDir = expandHome(args[1])
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot create output directory: %v\n", err)
		os.Exit(1)
	}

	client := mustLogin()
	fmt.Fprintf(os.Stderr, "Downloading torrent %s...\n", id)
	outPath, err := client.downloadTorrentFile(id, outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Saved: %s\n", outPath)
}

func cmdScan(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "error: scan requires a directory argument")
		os.Exit(1)
	}
	dir := args[0]

	series, err := scanMediaDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(series) == 0 {
		fmt.Println("No series directories found.")
		return
	}

	fmt.Printf("\nFound %d series in %s:\n\n", len(series), dir)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FOLDER\tSEARCH NAME\tEPISODES ON DISK")
	fmt.Fprintln(w, strings.Repeat("─", 30)+"\t"+strings.Repeat("─", 25)+"\t"+strings.Repeat("─", 16))
	for _, s := range series {
		fmt.Fprintf(w, "%s\t%s\t%d\n", s.FolderName, s.SearchName, len(s.Episodes))
	}
	w.Flush()
	fmt.Println()
}

func cmdWatch(args []string, install bool) {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	mediaDir := fs.String("media-dir", "", "Media directory to watch (required)")
	torrentDir := fs.String("torrent-dir", "", "Staging directory for .torrent files")
	stateFile := fs.String("state", "/var/lib/ncore-cli/state.json", "Path to state JSON file")
	interval := fs.Duration("interval", 6*time.Hour, "Check interval (e.g. 6h, 30m)")
	downloadCmd := fs.String("download-cmd", "webtorrent download {torrent} --out {out_dir}", "Download command template")
	once := fs.Bool("once", false, "Run one cycle then exit")
	require := fs.String("require", "", "Comma-separated terms the torrent name must contain (e.g. 1080p)")
	exclude := fs.String("exclude", "", "Comma-separated terms the torrent name must not contain (e.g. 2160p,2160,UHD)")
	_ = fs.Parse(args)

	if *mediaDir == "" {
		fmt.Fprintln(os.Stderr, "error: --media-dir is required")
		os.Exit(1)
	}
	if *torrentDir == "" {
		*torrentDir = filepath.Join(*mediaDir, "torrent")
	}

	cfg := WatchConfig{
		MediaDir:    *mediaDir,
		TorrentDir:  *torrentDir,
		StateFile:   *stateFile,
		Interval:    *interval,
		DownloadCmd: *downloadCmd,
		Once:        *once,
		Categories:  defaultWatchCategories(),
		Require:     splitCSV(*require),
		Exclude:     splitCSV(*exclude),
	}

	if install {
		// Credentials are optional at install time — they go into /etc/ncore-cli/env
		// which the user can edit before starting the service.
		cfg.NcoreUser = os.Getenv("NCORE_USER")
		cfg.NcorePass = os.Getenv("NCORE_PASS")
		if err := runInstall(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg.NcoreUser = mustEnv("NCORE_USER")
	cfg.NcorePass = mustEnv("NCORE_PASS")

	if err := runWatcher(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func mustLogin() *Client {
	username := mustEnv("NCORE_USER")
	password := mustEnv("NCORE_PASS")
	client, err := newClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "Logging in...")
	if err := client.login(username, password); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return client
}
