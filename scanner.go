package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	episodeFileRe  = regexp.MustCompile(`(?i)[Ss](\d{1,2})[Ee](\d{1,2})`)
	releaseSplitRe = regexp.MustCompile(`(?i)[Ss]\d{1,2}(?:[Ee]\d+|-[Ss]\d+|[._]|$)`)
	codecSplitRe   = regexp.MustCompile(`(?i)[._](480p|720p|1080p|2160p|4K|UHD|WEB[._-]DL|WEBRip|BluRay|BDRip|x264|x265|HEVC|COMPLETE|RERIP|REPACK)`)
	dotUnderRe     = regexp.MustCompile(`[._]+`)
)

// ScannedSeries holds what was found on disk for one series.
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

	// Deduplicate by search name — release-style folders share one entry.
	seriesMap := make(map[string]*ScannedSeries)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") || e.Name() == "torrent" {
			continue
		}
		folderPath := filepath.Join(dir, e.Name())

		if isReleaseName(e.Name()) {
			// Release-style: "Doc.2025.S02E01.720p.WEB-DL..."
			// Multiple such folders may belong to the same series.
			seriesName := extractSeriesName(e.Name())
			epKey := extractEpisodeKey(e.Name())

			s, exists := seriesMap[seriesName]
			if !exists {
				s = &ScannedSeries{
					FolderName: seriesName,
					FolderPath: dir, // new episodes go into the media root
					SearchName: seriesName,
					Episodes:   make(map[string]bool),
				}
				seriesMap[seriesName] = s
			}
			if epKey != "" {
				s.Episodes[epKey] = true
			}
			// Scan inside folder for additional episode files (multi-episode releases).
			for ep := range scanEpisodes(folderPath) {
				s.Episodes[ep] = true
			}
		} else {
			// Standard-style: "Breaking Bad" or "Breaking.Bad" with Season dirs inside.
			searchName := cleanFolderName(e.Name())
			s, exists := seriesMap[searchName]
			if !exists {
				s = &ScannedSeries{
					FolderName: e.Name(),
					FolderPath: folderPath,
					SearchName: searchName,
					Episodes:   make(map[string]bool),
				}
				seriesMap[searchName] = s
			}
			for ep := range scanEpisodes(folderPath) {
				s.Episodes[ep] = true
			}
		}
	}

	result := make([]ScannedSeries, 0, len(seriesMap))
	for _, s := range seriesMap {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].FolderName < result[j].FolderName
	})
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

// isReleaseName returns true when the folder looks like a release archive
// (contains a season/episode marker or known codec/quality keyword).
func isReleaseName(name string) bool {
	return releaseSplitRe.MatchString(name) || codecSplitRe.MatchString(name)
}

// extractSeriesName strips everything from the first season/episode or codec
// marker onward and returns a clean series name.
// "Doc.2025.S02E01.720p.WEB-DL..." → "Doc 2025"
// "Star.Trek.Starfleet.Academy.S01E01..." → "Star Trek Starfleet Academy"
func extractSeriesName(name string) string {
	if loc := releaseSplitRe.FindStringIndex(name); loc != nil {
		name = name[:loc[0]]
	} else if loc := codecSplitRe.FindStringIndex(name); loc != nil {
		name = name[:loc[0]]
	}
	s := dotUnderRe.ReplaceAllString(name, " ")
	return strings.TrimSpace(s)
}

// extractEpisodeKey pulls "S##E##" from a release folder name, or "" if absent.
func extractEpisodeKey(name string) string {
	m := episodeFileRe.FindStringSubmatch(name)
	if len(m) < 3 {
		return ""
	}
	s, _ := strconv.Atoi(m[1])
	e, _ := strconv.Atoi(m[2])
	return fmt.Sprintf("S%02dE%02d", s, e)
}

// cleanFolderName converts "Breaking.Bad" → "Breaking Bad".
func cleanFolderName(name string) string {
	s := dotUnderRe.ReplaceAllString(name, " ")
	return strings.TrimSpace(s)
}
