package infrastructure

import (
	"bytes"
	"fmt"
	"go.yaml.in/yaml/v3"
	harnessdomain "harnessforge/internal/harness/domain"
	"harnessforge/internal/inputlimits"
	"harnessforge/internal/safefile"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ReviewKnowledge updates one referenced Forge item. Approval binds the
// reviewer to current content and revalidates all evidence before publication.
func ReviewKnowledge(root, selection, id, state, reviewer string) error {
	if state != "candidate" && state != "approved" && state != "rejected" && state != "deprecated" {
		return fmt.Errorf("unsupported knowledge review state %q", state)
	}
	if state != "candidate" && strings.TrimSpace(reviewer) == "" {
		return fmt.Errorf("--reviewer is required when approving, rejecting, or deprecating knowledge")
	}
	project, err := LoadProject(root, selection)
	if err != nil {
		return err
	}
	if project.Manifest == nil {
		return fmt.Errorf("knowledge review requires the Forge layout")
	}
	var ref *harnessdomain.KnowledgeReference
	for i := range project.Manifest.References.Knowledge {
		if project.Manifest.References.Knowledge[i].ID == id {
			ref = &project.Manifest.References.Knowledge[i]
			break
		}
	}
	if ref == nil {
		return fmt.Errorf("knowledge item %q is not referenced by the Forge manifest", id)
	}
	path, err := project.Layout.ResolveReference(ref.Path)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("knowledge item must be a regular non-symlink file")
	}
	relative, err := filepath.Rel(project.Layout.Root, path)
	if err != nil {
		return err
	}
	relative = filepath.ToSlash(relative)
	rootHandle, err := os.OpenRoot(project.Layout.Root)
	if err != nil {
		return err
	}
	defer rootHandle.Close()
	if err := rejectSymlinkPath(rootHandle, relative); err != nil {
		return err
	}
	file, err := rootHandle.Open(filepath.FromSlash(relative))
	if err != nil {
		return err
	}
	original, err := io.ReadAll(io.LimitReader(file, inputlimits.HarnessYAMLBytes+1))
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if int64(len(original)) > inputlimits.HarnessYAMLBytes {
		return fmt.Errorf("knowledge document exceeds size limit")
	}
	text := string(original)
	opening := strings.Index(text, "---")
	if opening < 0 || strings.TrimSpace(text[:opening]) != "" {
		return fmt.Errorf("knowledge document must use YAML front matter")
	}
	contentStart := opening + len("---")
	closingOffset := strings.Index(text[contentStart:], "\n---")
	if closingOffset < 0 {
		return fmt.Errorf("unterminated knowledge front matter")
	}
	closingStart := contentStart + closingOffset + 1
	closingEnd := closingStart + len("---")
	if closingEnd < len(text) && text[closingEnd] != '\n' && text[closingEnd] != '\r' {
		return fmt.Errorf("invalid knowledge front matter delimiter")
	}
	decoder := yaml.NewDecoder(strings.NewReader(strings.TrimSpace(text[contentStart:closingStart])))
	decoder.KnownFields(true)
	var item harnessdomain.KnowledgeItem
	if err := decoder.Decode(&item); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one YAML document")
	}
	if item.ID != id {
		return fmt.Errorf("manifest ID %q does not match document ID %q", id, item.ID)
	}
	// A review decision can only be made once from the candidate state. Check
	// the source state before changing it; comparing after assignment would
	// compare the requested state with itself and allow repeat reviews.
	if state != "candidate" && item.Review != harnessdomain.KnowledgeCandidate {
		return fmt.Errorf("only candidate knowledge can enter a new review")
	}
	if state == "candidate" {
		item.Review = harnessdomain.KnowledgeCandidate
		item.Reviewer = ""
		item.ReviewDiff = ""
		item.ContentSHA256 = ""
		item.EvidenceSHA256 = ""
		item.Health = harnessdomain.KnowledgeUnknown
	} else {
		item.Review = harnessdomain.KnowledgeReviewState(state)
		item.Reviewer = strings.TrimSpace(reviewer)
		item.ReviewDiff = reviewDiff(item.Content)
		item.ContentSHA256 = harnessdomain.HashKnowledgeContent(item.Content)
		fingerprint, err := KnowledgeFingerprint(project.Layout.Root, path, id, item.Evidence)
		if err != nil {
			return fmt.Errorf("knowledge evidence is invalid; review refused: %w", err)
		}
		item.EvidenceSHA256 = fingerprint
		if state == "approved" {
			item.Health = harnessdomain.KnowledgeVerified
		} else {
			item.Health = harnessdomain.KnowledgeUnknown
		}
	}
	if err := item.Validate(); err != nil {
		return err
	}
	encoded, err := yaml.Marshal(item)
	if err != nil {
		return err
	}
	body := text[closingEnd:]
	updated := append(append([]byte("---\n"), encoded...), []byte("---"+body)...)
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, original) {
		return fmt.Errorf("knowledge item changed during review")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "forge-knowledge-review-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(updated); err == nil {
		err = tmp.Chmod(info.Mode().Perm())
	}
	if err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err != nil {
		return err
	}
	return safefile.Replace(name, path)
}

func reviewDiff(content string) string {
	lines := strings.Split(content, "\n")
	var out strings.Builder
	// Avoid YAML front-matter delimiter tokens: several lightweight readers
	// locate the closing delimiter by scanning for `---`.
	out.WriteString("candidate content reviewed:\n")
	for _, line := range lines {
		fmt.Fprintf(&out, "+%s\n", line)
	}
	return out.String()
}

func rejectSymlinkPath(root *os.Root, relative string) error {
	clean := filepath.Clean(filepath.FromSlash(relative))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("knowledge path escapes repository root")
	}
	current := "."
	parts := strings.Split(filepath.ToSlash(clean), "/")
	for i, part := range parts {
		current = filepath.Join(current, filepath.FromSlash(part))
		info, err := root.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("knowledge path contains symlink")
		}
		if i < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("knowledge parent is not a directory")
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return fmt.Errorf("knowledge item is not a regular file")
		}
	}
	return nil
}
