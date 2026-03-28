package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var episodeFileRe = regexp.MustCompile(`(?i)[Ss](\d{1,2})[Ee](\d{1,2})`)

// ScannedSeries holds what was found on disk for one series folder.
type ScannedSeries struct {
	FolderName string
	FolderPath string
	SearchName string          // cleaned name to use for TVMaze lookup
	Episodes   map[string]bool // "S01E01" → true
}

func scanMediaDir(dir string) ([]ScannedSeries, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	var result []ScannedSeries
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Skip hidden dirs and the torrent staging dir
		if strings.HasPrefix(e.Name(), ".") || e.Name() == "torrent" {
			continue
		}

		seriesPath := filepath.Join(dir, e.Name())
		result = append(result, ScannedSeries{
			FolderName: e.Name(),
			FolderPath: seriesPath,
			SearchName: cleanFolderName(e.Name()),
			Episodes:   scanEpisodes(seriesPath),
		})
	}
	return result, nil
}

func scanEpisodes(dir string) map[string]bool {
	found := make(map[string]bool)
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := filepath.Base(path)
		if m := episodeFileRe.FindStringSubmatch(name); len(m) == 3 {
			s, _ := strconv.Atoi(m[1])
			e, _ := strconv.Atoi(m[2])
			found[fmt.Sprintf("S%02dE%02d", s, e)] = true
		}
		return nil
	})
	return found
}

var (
	dotUnderRe  = regexp.MustCompile(`[._]+`)
	trailingYear = regexp.MustCompile(`\s+\d{4}$`)
)

// cleanFolderName converts "Breaking.Bad" → "Breaking Bad", strips trailing years.
func cleanFolderName(name string) string {
	s := dotUnderRe.ReplaceAllString(name, " ")
	s = strings.TrimSpace(s)
	s = trailingYear.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
