package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
)

const (
	baseURL   = "https://ncore.pro"
	loginURL  = baseURL + "/login.php"
	searchURL = baseURL + "/torrents.php"
)

// Client holds a logged-in HTTP session and the user's passkey.
type Client struct {
	http    *http.Client
	passkey string
}

// Torrent represents a single search result.
type Torrent struct {
	ID      string
	Name    string
	Size    string
	Seeds   string
	Leeches string
	Type    string
}

func newClient() (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cookie jar: %w", err)
	}
	return &Client{
		http: &http.Client{Jar: jar},
	}, nil
}

// passKeyRe matches the user passkey from the RSS link or the JS torrent() function.
var passKeyRe = regexp.MustCompile(`key=([a-f0-9]{32})`)

func (c *Client) login(username, password string) error {
	resp, err := c.http.PostForm(loginURL, url.Values{
		"nev":             {username},
		"pass":            {password},
		"ne_leptessen_ki": {"1"},
		"submitted":       {"1"},
	})
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	bodyStr := string(body)

	// Still showing the login form → credentials rejected
	if strings.Contains(bodyStr, `name="nev"`) {
		if m := regexp.MustCompile(`class="hibauzenet"[^>]*>([^<]+)<`).FindStringSubmatch(bodyStr); len(m) == 2 {
			return fmt.Errorf("login failed: %s", strings.TrimSpace(m[1]))
		}
		return fmt.Errorf("login failed: invalid credentials")
	}

	// Extract passkey from the RSS alternate link: href="/rss.php?key=<hex>"
	if m := passKeyRe.FindStringSubmatch(bodyStr); len(m) == 2 {
		c.passkey = m[1]
	}

	return nil
}

var (
	// onclick="torrent(1234567)"
	onclickIDRe = regexp.MustCompile(`onclick="torrent\((\d+)\)`)
	// action=details&id=1234567 in href
	detailsIDRe = regexp.MustCompile(`action=details&(?:amp;)?id=(\d+)`)
	// title="Full Name Here"
	titleRe = regexp.MustCompile(`action=details[^"]*"\s[^>]*title="([^"]+)"`)
	// category alt from box_alap_img: alt="HD/HU" ((?s) makes . match newlines)
	catAltRe = regexp.MustCompile(`(?s)class="box_alap_img"[^>]*>.*?alt="([^"]+)"`)
	// <div class="box_meret2">159.99 MiB</div>
	sizeRe = regexp.MustCompile(`class="box_meret2">([^<]+)<`)
	// <div class="box_s2"><a ...>77</a></div>
	seedsRe = regexp.MustCompile(`class="box_s2"><a[^>]+>(\d+)<`)
	// <div class="box_l2"><a ...>0</a></div>
	leechesRe = regexp.MustCompile(`class="box_l2"><a[^>]+>(\d+)<`)
)

// parseTorrents parses ncore search result HTML into Torrent entries.
// Each result is wrapped in <div class="box_torrent">.
func parseTorrents(body string) []Torrent {
	var results []Torrent

	// Split on the exact opening of each torrent block.
	// "box_torrent_all" wraps all results; "box_torrent" (with closing ") is each entry.
	const marker = `class="box_torrent"`
	parts := strings.Split(body, marker)

	for _, part := range parts[1:] {
		// Skip the "box_torrent_all" container — it starts with `_all`
		if strings.HasPrefix(strings.TrimLeft(part, " \t\r\n"), "_all") {
			continue
		}

		// Grab content up to the next block boundary (torrent_lenyilo div closes the block)
		end := strings.Index(part, `class="torrent_lenyilo`)
		if end != -1 {
			part = part[:end]
		}

		t := Torrent{}

		// ID — prefer onclick="torrent(ID)" as it's the most reliable
		if m := onclickIDRe.FindStringSubmatch(part); len(m) == 2 {
			t.ID = m[1]
		} else if m := detailsIDRe.FindStringSubmatch(part); len(m) == 2 {
			t.ID = m[1]
		}

		// Full name from title attribute of the details link
		if m := titleRe.FindStringSubmatch(part); len(m) == 2 {
			t.Name = m[1]
		}

		// Category from the alt attribute of the category icon
		if m := catAltRe.FindStringSubmatch(part); len(m) == 2 {
			t.Type = m[1]
		}

		// Size
		if m := sizeRe.FindStringSubmatch(part); len(m) == 2 {
			t.Size = strings.TrimSpace(m[1])
		}

		// Seeds
		if m := seedsRe.FindStringSubmatch(part); len(m) == 2 {
			t.Seeds = m[1]
		}

		// Leeches
		if m := leechesRe.FindStringSubmatch(part); len(m) == 2 {
			t.Leeches = m[1]
		}

		if t.ID != "" {
			results = append(results, t)
		}
	}

	return results
}

func (c *Client) search(query, category string) ([]Torrent, error) {
	params := url.Values{
		"miben":  {"name"},
		"mire":   {query},
		"tipus":  {category},
		"letolt": {"true"},
		"keret":  {"1"},
	}

	resp, err := c.http.Get(searchURL + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Also update passkey if we haven't gotten it yet
	if c.passkey == "" {
		if m := passKeyRe.FindStringSubmatch(string(body)); len(m) == 2 {
			c.passkey = m[1]
		}
	}

	return parseTorrents(string(body)), nil
}

func (c *Client) download(id, outDir string) (string, error) {
	dlURL := fmt.Sprintf("%s?action=download&id=%s", searchURL, id)
	if c.passkey != "" {
		dlURL += "&key=" + c.passkey
	}

	resp, err := c.http.Get(dlURL)
	if err != nil {
		return "", fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Determine filename from Content-Disposition, fall back to <id>.torrent
	filename := id + ".torrent"
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		for _, part := range strings.Split(cd, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "filename=") {
				filename = strings.Trim(strings.TrimPrefix(part, "filename="), `"'`)
			}
		}
	}

	outPath := filepath.Join(outDir, filename)
	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return outPath, nil
}

func usage() {
	fmt.Fprint(os.Stderr, `ncore-cli — search and download torrents from ncore.pro

Environment variables:
  NCORE_USER   ncore.pro username (required)
  NCORE_PASS   ncore.pro password (required)

Commands:
  search <query> [category]
      Search torrents. Category defaults to "all_own".
      Common categories: all_own, xvid_hun, xvid, dvd_hun, dvd,
        dvd9_hun, dvd9, hd_hun, hd, hdser_hun, hdser, mp3_hun, mp3,
        lossless_hun, lossless, ebook_hun, ebook, game_iso, game_rip,
        console, xxx_xvid, xxx_dvd, xxx_hd, xxx_imageset, misc, mobil

  download <id> [output_dir]
      Download the torrent file for the given ID.
      Output directory defaults to the current working directory.

Examples:
  ncore-cli search "inception" hd_hun
  ncore-cli download 3995642
  ncore-cli download 3995642 ~/Downloads
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

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		usage()
		if len(os.Args) < 2 {
			os.Exit(1)
		}
		return
	}

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

	switch os.Args[1] {
	case "search":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "error: search requires a query argument")
			usage()
			os.Exit(1)
		}
		query := os.Args[2]
		category := "all_own"
		if len(os.Args) >= 4 {
			category = os.Args[3]
		}

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
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				t.ID, name, t.Size, t.Seeds, t.Leeches, t.Type)
		}
		w.Flush()
		fmt.Println()

	case "download":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "error: download requires an ID argument")
			usage()
			os.Exit(1)
		}
		id := os.Args[2]
		outDir := "."
		if len(os.Args) >= 4 {
			outDir = expandHome(os.Args[3])
		}

		if err := os.MkdirAll(outDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot create output directory: %v\n", err)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stderr, "Downloading torrent %s...\n", id)
		outPath, err := client.download(id, outDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Saved: %s\n", outPath)

	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}
