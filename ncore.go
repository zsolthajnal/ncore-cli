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

	if strings.Contains(bodyStr, `name="nev"`) {
		if m := regexp.MustCompile(`class="hibauzenet"[^>]*>([^<]+)<`).FindStringSubmatch(bodyStr); len(m) == 2 {
			return fmt.Errorf("login failed: %s", strings.TrimSpace(m[1]))
		}
		return fmt.Errorf("login failed: invalid credentials")
	}

	if m := passKeyRe.FindStringSubmatch(bodyStr); len(m) == 2 {
		c.passkey = m[1]
	}

	return nil
}

var (
	onclickIDRe = regexp.MustCompile(`onclick="torrent\((\d+)\)`)
	detailsIDRe = regexp.MustCompile(`action=details&(?:amp;)?id=(\d+)`)
	titleRe     = regexp.MustCompile(`action=details[^"]*"\s[^>]*title="([^"]+)"`)
	catAltRe    = regexp.MustCompile(`(?s)class="box_alap_img"[^>]*>.*?alt="([^"]+)"`)
	sizeRe      = regexp.MustCompile(`class="box_meret2">([^<]+)<`)
	seedsRe     = regexp.MustCompile(`class="box_s2"><a[^>]+>(\d+)<`)
	leechesRe   = regexp.MustCompile(`class="box_l2"><a[^>]+>(\d+)<`)
)

func parseTorrents(body string) []Torrent {
	var results []Torrent
	const marker = `class="box_torrent"`
	parts := strings.Split(body, marker)

	for _, part := range parts[1:] {
		if strings.HasPrefix(strings.TrimLeft(part, " \t\r\n"), "_all") {
			continue
		}
		if end := strings.Index(part, `class="torrent_lenyilo`); end != -1 {
			part = part[:end]
		}

		t := Torrent{}
		if m := onclickIDRe.FindStringSubmatch(part); len(m) == 2 {
			t.ID = m[1]
		} else if m := detailsIDRe.FindStringSubmatch(part); len(m) == 2 {
			t.ID = m[1]
		}
		if m := titleRe.FindStringSubmatch(part); len(m) == 2 {
			t.Name = m[1]
		}
		if m := catAltRe.FindStringSubmatch(part); len(m) == 2 {
			t.Type = m[1]
		}
		if m := sizeRe.FindStringSubmatch(part); len(m) == 2 {
			t.Size = strings.TrimSpace(m[1])
		}
		if m := seedsRe.FindStringSubmatch(part); len(m) == 2 {
			t.Seeds = m[1]
		}
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
	if c.passkey == "" {
		if m := passKeyRe.FindStringSubmatch(string(body)); len(m) == 2 {
			c.passkey = m[1]
		}
	}
	return parseTorrents(string(body)), nil
}

func (c *Client) downloadTorrentFile(id, outDir string) (string, error) {
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
