package manifest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type QueueEntry struct {
	Slug   string
	Status string
	Index  int // 0 means unsorted
}

func LoadSlugs(ticketsDir string) (map[string]string, error) {
	result := make(map[string]string)
	f, err := os.Open(filepath.Join(ticketsDir, "chipper-slugs"))
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return result, scanner.Err()
}

func LoadQueue(ticketsDir string) ([]QueueEntry, error) {
	var entries []QueueEntry
	f, err := os.Open(filepath.Join(ticketsDir, "chipper-queue"))
	if os.IsNotExist(err) {
		return entries, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// format: slug = status [index]
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		slug := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 1 {
			continue
		}
		entry := QueueEntry{
			Slug:   slug,
			Status: fields[0],
		}
		if len(fields) >= 2 {
			if idx, err := strconv.Atoi(fields[1]); err == nil {
				entry.Index = idx
			}
		}
		entries = append(entries, entry)
	}

	// Preserve file order for completed entries; sort active entries by index
	// by delegating to SaveQueue's sectioning logic on next write.
	return entries, scanner.Err()
}

func IsTerminal(status string) bool {
	return status == "done" || status == "cancelled" || status == "archived"
}

func SaveQueue(ticketsDir string, entries []QueueEntry) error {
	var prioritized, unsorted, completed []QueueEntry
	for _, e := range entries {
		e := e
		if IsTerminal(e.Status) {
			e.Index = 0
			completed = append(completed, e)
		} else if e.Index > 0 {
			prioritized = append(prioritized, e)
		} else {
			unsorted = append(unsorted, e)
		}
	}

	sort.Slice(prioritized, func(i, j int) bool {
		return prioritized[i].Index < prioritized[j].Index
	})

	f, err := os.Create(filepath.Join(ticketsDir, "chipper-queue"))
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, e := range prioritized {
		fmt.Fprintf(w, "%-24s= %-16s %d\n", e.Slug, e.Status, e.Index)
	}
	for _, e := range unsorted {
		fmt.Fprintf(w, "%-24s= %s\n", e.Slug, e.Status)
	}
	if len(completed) > 0 {
		fmt.Fprintln(w)
		for _, e := range completed {
			fmt.Fprintf(w, "%-24s= %s\n", e.Slug, e.Status)
		}
	}
	return w.Flush()
}

func FindBySlug(entries []QueueEntry, slug string) *QueueEntry {
	for i := range entries {
		if entries[i].Slug == slug {
			return &entries[i]
		}
	}
	return nil
}

func FindInProgress(entries []QueueEntry) *QueueEntry {
	for i := range entries {
		if entries[i].Status == "in_progress" {
			return &entries[i]
		}
	}
	return nil
}

// reservedFiles are manifest files that live in the tickets directory
// and should not be treated as tickets.
var reservedFiles = map[string]bool{
	"chipper-slugs":        true,
	"chipper-queue":        true,
	"chipper-dependencies": true,
}

// UnregisteredFiles returns ticket files in ticketsDir that have no slug assigned.
func UnregisteredFiles(ticketsDir string) ([]string, error) {
	slugs, err := LoadSlugs(ticketsDir)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(ticketsDir)
	if err != nil {
		return nil, err
	}

	var unregistered []string
	for _, e := range entries {
		if e.IsDir() || reservedFiles[e.Name()] {
			continue
		}
		if _, ok := slugs[e.Name()]; !ok {
			unregistered = append(unregistered, e.Name())
		}
	}
	return unregistered, nil
}

// SaveSlugs writes the filename->slug mapping to chipper-slugs.
func SaveSlugs(ticketsDir string, slugs map[string]string) error {
	f, err := os.Create(filepath.Join(ticketsDir, "chipper-slugs"))
	if err != nil {
		return err
	}
	defer f.Close()

	// Collect and sort keys for stable output
	keys := make([]string, 0, len(slugs))
	for k := range slugs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	w := bufio.NewWriter(f)
	for _, k := range keys {
		fmt.Fprintf(w, "%s = %s\n", k, slugs[k])
	}
	return w.Flush()
}

// SlugTaken returns true if the slug is already used in the map.
func SlugTaken(slugs map[string]string, slug string) bool {
	for _, v := range slugs {
		if v == slug {
			return true
		}
	}
	return false
}

// AddToQueue appends a slug with a given status and no sort index (unsorted).
func AddToQueue(entries []QueueEntry, slug, status string) []QueueEntry {
	return append(entries, QueueEntry{Slug: slug, Status: status})
}

// Head returns the highest-priority active ticket (lowest non-zero index),
// falling back to the first unsorted active entry if none are indexed.
func Head(entries []QueueEntry) *QueueEntry {
	for i := range entries {
		if entries[i].Index > 0 && !IsTerminal(entries[i].Status) {
			return &entries[i]
		}
	}
	for i := range entries {
		if !IsTerminal(entries[i].Status) {
			return &entries[i]
		}
	}
	return nil
}

// TopN returns up to n active tickets in priority order.
func TopN(entries []QueueEntry, n int) []QueueEntry {
	var out []QueueEntry
	for _, e := range entries {
		if !IsTerminal(e.Status) {
			out = append(out, e)
		}
	}
	if n < len(out) {
		out = out[:n]
	}
	return out
}

// UnsortedEntries returns active queue entries that have no priority index.
func UnsortedEntries(entries []QueueEntry) []QueueEntry {
	var out []QueueEntry
	for _, e := range entries {
		if e.Index == 0 && !IsTerminal(e.Status) {
			out = append(out, e)
		}
	}
	return out
}

// OrphanedSlugs returns slugs or queue entries whose ticket file no longer exists on disk.
func OrphanedSlugs(ticketsDir string, slugs map[string]string, queue []QueueEntry) []string {
	slugToFile := make(map[string]string)
	for filename, slug := range slugs {
		slugToFile[slug] = filename
	}

	seen := make(map[string]bool)
	var out []string

	for filename, slug := range slugs {
		if _, err := os.Stat(filepath.Join(ticketsDir, filename)); os.IsNotExist(err) {
			if !seen[slug] {
				out = append(out, slug)
				seen[slug] = true
			}
		}
	}
	for _, e := range queue {
		if seen[e.Slug] {
			continue
		}
		if _, ok := slugToFile[e.Slug]; !ok {
			out = append(out, e.Slug)
			seen[e.Slug] = true
		}
	}
	return out
}

// SlugToFile returns a reverse map from slug to filename.
func SlugToFile(slugs map[string]string) map[string]string {
	out := make(map[string]string, len(slugs))
	for filename, slug := range slugs {
		out[slug] = filename
	}
	return out
}

func UpdateStatus(entries []QueueEntry, slug, status string) ([]QueueEntry, error) {
	for i := range entries {
		if entries[i].Slug == slug {
			entries[i].Status = status
			return entries, nil
		}
	}
	return entries, fmt.Errorf("ticket %q not found in queue", slug)
}
