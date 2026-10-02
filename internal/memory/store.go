// Package memory stores explicit, local-only observations for one repository
// checkout. It never reads agent sessions or publishes observations implicitly.
package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"harnessforge/internal/safefile"
)

type State string

const (
	Candidate State = "candidate"
	Approved  State = "approved"
	Rejected  State = "rejected"
)

type Evidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
	Quote  string `json:"quote,omitempty"`
}
type Observation struct {
	ID            string     `json:"id"`
	RepositoryID  string     `json:"repository_id"`
	CheckoutID    string     `json:"checkout_id"`
	Revision      string     `json:"revision,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	State         State      `json:"state"`
	Content       string     `json:"content"`
	ContentSHA256 string     `json:"content_sha256"`
	RecordSHA256  string     `json:"record_sha256"`
	Source        string     `json:"source"`
	Evidence      []Evidence `json:"evidence,omitempty"`
	Reviewer      string     `json:"reviewer,omitempty"`
	ReviewedAt    time.Time  `json:"reviewed_at,omitempty"`
	ReviewDiff    string     `json:"review_diff,omitempty"`
}

type Store struct {
	root           string
	repositoryRoot string
	repositoryID   string
	checkoutID     string
	replaceFile    func(temporary, destination string) error
}

// Open uses the platform's persistent configuration directory, not a cache.
// Repository identity is based on the git common directory; checkout identity
// is the canonical worktree path so observations never cross worktrees.
func Open(repository string) (*Store, error) {
	root, err := filepath.Abs(repository)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("repository must be a directory")
	}
	common := root
	if output, e := exec.Command("git", "-C", root, "rev-parse", "--path-format=absolute", "--git-common-dir").Output(); e == nil {
		common = strings.TrimSpace(string(output))
		if !filepath.IsAbs(common) {
			common = filepath.Join(root, common)
		}
		if resolved, e := filepath.EvalSymlinks(common); e == nil {
			common = resolved
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(base) == "" {
		return nil, fmt.Errorf("persistent user configuration directory unavailable")
	}
	return &Store{root: filepath.Join(base, "harnessforge", "memory", hashIdentity(common), hashIdentity(root)), repositoryRoot: root, repositoryID: hashIdentity(common), checkoutID: hashIdentity(root)}, nil
}

func (s *Store) Capture(content, source string, evidence []Evidence) (Observation, error) {
	content = strings.TrimSpace(content)
	source = strings.TrimSpace(source)
	if content == "" || source == "" {
		return Observation{}, fmt.Errorf("observation content and explicit source are required")
	}
	if len(content) > 1<<20 {
		return Observation{}, fmt.Errorf("observation content exceeds 1 MiB")
	}
	if len(evidence) > 100 {
		return Observation{}, fmt.Errorf("observation has too many evidence references")
	}
	for _, item := range evidence {
		clean := filepath.Clean(filepath.FromSlash(item.Path))
		if item.Path == "" || filepath.IsAbs(item.Path) || strings.Contains(item.Path, "\\") || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return Observation{}, fmt.Errorf("evidence paths must be repository-relative")
		}
	}
	lock, err := s.lockMutation()
	if err != nil {
		return Observation{}, err
	}
	defer lock.Close()
	info, err := os.Stat(filepath.Join(s.root, "observations.json"))
	if err == nil && info.Size() > 8<<20 {
		return Observation{}, fmt.Errorf("local observations exceed 8 MiB; review or collect garbage before capturing more")
	}
	entries, err := s.list()
	if err != nil {
		return Observation{}, err
	}
	sum := sha256.Sum256([]byte(content))
	now := time.Now().UTC()
	idSum := sha256.Sum256([]byte(s.repositoryID + "\x00" + s.checkoutID + "\x00" + now.Format(time.RFC3339Nano) + "\x00" + hex.EncodeToString(sum[:])))
	revision := gitRevision(s.repositoryRoot)
	item := Observation{ID: hex.EncodeToString(idSum[:16]), RepositoryID: s.repositoryID, CheckoutID: s.checkoutID, Revision: revision, CreatedAt: now, State: Candidate, Content: content, ContentSHA256: hex.EncodeToString(sum[:]), Source: source, Evidence: append([]Evidence(nil), evidence...)}
	for _, existing := range entries {
		if existing.ContentSHA256 == item.ContentSHA256 && existing.Source == item.Source && existing.State == Candidate {
			return existing, nil
		}
	}
	entries = append(entries, item)
	if err := s.save(entries); err != nil {
		return Observation{}, err
	}
	return item, nil
}

func (s *Store) List() ([]Observation, error) {
	return s.list()
}

// list reads a verified snapshot without taking the mutation lock. Atomic
// replacement in save means readers see either the complete old or new file.
func (s *Store) list() ([]Observation, error) {
	data, err := os.ReadFile(filepath.Join(s.root, "observations.json"))
	if os.IsNotExist(err) {
		return []Observation{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > 8<<20 {
		return nil, fmt.Errorf("local observations exceed 8 MiB")
	}
	var entries []Observation
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("local observation store is invalid: %w", err)
	}
	for _, item := range entries {
		sum := sha256.Sum256([]byte(item.Content))
		recordHash := item.RecordSHA256
		item.RecordSHA256 = ""
		encoded, _ := json.Marshal(item)
		recordSum := sha256.Sum256(encoded)
		if item.RepositoryID != s.repositoryID || item.CheckoutID != s.checkoutID || hex.EncodeToString(sum[:]) != item.ContentSHA256 || recordHash != hex.EncodeToString(recordSum[:]) {
			return nil, fmt.Errorf("local observation integrity or checkout identity mismatch")
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].CreatedAt.Equal(entries[j].CreatedAt) {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].CreatedAt.Before(entries[j].CreatedAt)
	})
	return entries, nil
}

func (s *Store) Review(id string, state State, reviewer string) (Observation, error) {
	if state != Candidate && state != Approved && state != Rejected {
		return Observation{}, fmt.Errorf("unsupported observation state %q", state)
	}
	if state != Candidate && strings.TrimSpace(reviewer) == "" {
		return Observation{}, fmt.Errorf("reviewer is required for a final decision")
	}
	lock, err := s.lockMutation()
	if err != nil {
		return Observation{}, err
	}
	defer lock.Close()
	entries, err := s.list()
	if err != nil {
		return Observation{}, err
	}
	for i := range entries {
		if entries[i].ID != id {
			continue
		}
		if entries[i].State == Approved || entries[i].State == Rejected {
			return Observation{}, fmt.Errorf("observation already has a final decision")
		}
		entries[i].State = state
		entries[i].Reviewer = strings.TrimSpace(reviewer)
		if state != Candidate {
			entries[i].ReviewedAt = time.Now().UTC()
			entries[i].ReviewDiff = "--- candidate\n+++ reviewed\n" + strings.Join(prefixLines(entries[i].Content), "\n") + "\n"
		}
		if err := s.save(entries); err != nil {
			return Observation{}, err
		}
		return entries[i], nil
	}
	return Observation{}, fmt.Errorf("observation %q not found", id)
}

func prefixLines(content string) []string {
	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = "+" + lines[i]
	}
	return lines
}

type GCPlan struct {
	Remove          []string `json:"remove"`
	PreservePending int      `json:"preserve_pending"`
	BytesReclaimed  int64    `json:"bytes_reclaimed"`
	QuotaExceeded   bool     `json:"quota_exceeded"`
}

func (s *Store) PreviewGC(now time.Time, retention time.Duration, maximum ...int) (GCPlan, error) {
	entries, err := s.List()
	if err != nil {
		return GCPlan{}, err
	}
	return previewGC(entries, now, retention, maximum...), nil
}

func previewGC(entries []Observation, now time.Time, retention time.Duration, maximum ...int) GCPlan {
	plan := GCPlan{Remove: []string{}}
	resolved := []Observation{}
	remove := map[string]bool{}
	for _, item := range entries {
		if item.State == Candidate {
			plan.PreservePending++
			continue
		}
		if retention > 0 && now.Sub(item.CreatedAt) > retention {
			plan.Remove = append(plan.Remove, item.ID)
			remove[item.ID] = true
			plan.BytesReclaimed += int64(len(item.Content))
		} else {
			resolved = append(resolved, item)
		}
	}
	limit := 0
	if len(maximum) > 0 {
		limit = maximum[0]
	}
	if limit > 0 {
		remaining := len(entries) - len(remove)
		for _, item := range resolved {
			if remaining <= limit {
				break
			}
			remove[item.ID] = true
			plan.Remove = append(plan.Remove, item.ID)
			plan.BytesReclaimed += int64(len(item.Content))
			remaining--
		}
		plan.QuotaExceeded = remaining > limit
	}
	sort.Strings(plan.Remove)
	return plan
}
func (s *Store) ApplyGC(now time.Time, retention time.Duration, maximum ...int) (GCPlan, error) {
	lock, err := s.lockMutation()
	if err != nil {
		return GCPlan{}, err
	}
	defer lock.Close()
	entries, err := s.list()
	if err != nil {
		return GCPlan{}, err
	}
	plan := previewGC(entries, now, retention, maximum...)
	remove := map[string]bool{}
	for _, id := range plan.Remove {
		remove[id] = true
	}
	kept := entries[:0]
	for _, item := range entries {
		if !remove[item.ID] {
			kept = append(kept, item)
		}
	}
	// GC is one atomic snapshot replacement. If the process stops before the
	// rename, the previous snapshot remains authoritative; if it stops after,
	// the complete new snapshot is authoritative. Calling ApplyGC again
	// recomputes the plan from that snapshot, making recovery idempotent.
	return plan, s.save(kept)
}

func (s *Store) save(entries []Observation) error {
	if err := os.MkdirAll(filepath.Dir(s.root), 0700); err != nil {
		return err
	}
	if err := os.Mkdir(s.root, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	info, err := os.Lstat(s.root)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("memory store path must be a regular directory")
	}
	for i := range entries {
		entries[i].RecordSHA256 = ""
		encoded, err := json.Marshal(entries[i])
		if err != nil {
			return err
		}
		sum := sha256.Sum256(encoded)
		entries[i].RecordSHA256 = hex.EncodeToString(sum[:])
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(s.root, "observations-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(0600)
	}
	if err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err != nil {
		return err
	}
	replace := s.replaceFile
	if replace == nil {
		replace = safefile.Replace
	}
	return replace(name, filepath.Join(s.root, "observations.json"))
}
func hashIdentity(value string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(value)))
	return hex.EncodeToString(sum[:])
}
func gitRevision(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
