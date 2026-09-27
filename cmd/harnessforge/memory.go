package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	harnessdomain "harnessforge/internal/harness/domain"
	harnessinfra "harnessforge/internal/harness/infrastructure"
	"harnessforge/internal/memory"
)

func newMemoryCommand() *cobra.Command {
	root := &cobra.Command{Use: "memory", Short: "Manage explicitly captured local observations"}
	var repository, source, reviewer string
	var evidencePaths []string
	capture := &cobra.Command{Use: "capture [content]", Short: "Capture an observation as a local candidate", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		store, err := memory.Open(repository)
		if err != nil {
			return err
		}
		var evidence []memory.Evidence
		for _, name := range evidencePaths {
			if filepath.IsAbs(name) {
				return fmt.Errorf("evidence must be repository-relative")
			}
			clean := filepath.Clean(name)
			if clean == ".." || filepath.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return fmt.Errorf("evidence path escapes repository")
			}
			root, err := filepath.Abs(repository)
			if err != nil {
				return err
			}
			root, err = filepath.EvalSymlinks(root)
			if err != nil {
				return err
			}
			handle, err := os.OpenRoot(root)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, filepath.Join(root, clean))
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				handle.Close()
				return fmt.Errorf("evidence escapes repository")
			}
			current := "."
			for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
				current = filepath.Join(current, part)
				info, statErr := handle.Lstat(current)
				if statErr != nil {
					handle.Close()
					return statErr
				}
				if info.Mode()&os.ModeSymlink != 0 {
					handle.Close()
					return fmt.Errorf("evidence must not contain symlink paths")
				}
				if current != rel && !info.IsDir() {
					handle.Close()
					return fmt.Errorf("evidence parent must be a directory")
				}
			}
			file, err := handle.Open(rel)
			if err != nil {
				handle.Close()
				return err
			}
			info, err := file.Stat()
			if err != nil {
				file.Close()
				handle.Close()
				return err
			}
			if !info.Mode().IsRegular() || info.Size() > 4<<20 {
				file.Close()
				handle.Close()
				return fmt.Errorf("evidence exceeds 4 MiB")
			}
			data, err := io.ReadAll(io.LimitReader(file, (4<<20)+1))
			file.Close()
			handle.Close()
			if err != nil {
				return err
			}
			if len(data) > 4<<20 {
				return fmt.Errorf("evidence exceeds 4 MiB")
			}
			sum := sha256.Sum256(data)
			evidence = append(evidence, memory.Evidence{Path: filepath.ToSlash(clean), SHA256: hex.EncodeToString(sum[:])})
		}
		item, err := store.Capture(args[0], source, evidence)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
	}}
	capture.Flags().StringVar(&repository, "repository", ".", "Repository checkout for this local observation")
	capture.Flags().StringVar(&source, "source", "", "Explicit capture source (for example manual or command-result)")
	capture.Flags().StringSliceVar(&evidencePaths, "evidence", nil, "Repository-relative evidence file (repeatable)")
	list := &cobra.Command{Use: "list", Short: "List local observations for this checkout", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		store, err := memory.Open(repository)
		if err != nil {
			return err
		}
		items, err := store.List()
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}}
	list.Flags().StringVar(&repository, "repository", ".", "Repository checkout")
	var retention time.Duration
	var apply bool
	var maxItems int
	gc := &cobra.Command{Use: "gc", Short: "Preview or remove expired resolved local observations", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		store, err := memory.Open(repository)
		if err != nil {
			return err
		}
		if retention <= 0 && maxItems <= 0 {
			return fmt.Errorf("set a positive --older-than or --max-items")
		}
		var plan memory.GCPlan
		if apply {
			plan, err = store.ApplyGC(time.Now(), retention, maxItems)
		} else {
			plan, err = store.PreviewGC(time.Now(), retention, maxItems)
		}
		if err != nil {
			return err
		}
		if err := json.NewEncoder(cmd.OutOrStdout()).Encode(plan); err != nil {
			return err
		}
		if !apply {
			fmt.Fprintln(cmd.ErrOrStderr(), "Preview only. Pass --apply to remove these resolved observations.")
		}
		return nil
	}}
	gc.Flags().StringVar(&repository, "repository", ".", "Repository checkout")
	gc.Flags().DurationVar(&retention, "older-than", 0, "Retention age for resolved observations")
	gc.Flags().IntVar(&maxItems, "max-items", 0, "Maximum local observation count; pending items are always preserved")
	gc.Flags().BoolVar(&apply, "apply", false, "Apply the previewed garbage collection")
	review := &cobra.Command{Use: "review [observation-id] [approved|rejected]", Short: "Record an explicit local observation review", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		store, err := memory.Open(repository)
		if err != nil {
			return err
		}
		item, err := store.Review(args[0], memory.State(args[1]), reviewer)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
	}}
	review.Flags().StringVar(&repository, "repository", ".", "Repository checkout")
	review.Flags().StringVar(&reviewer, "reviewer", "", "Human reviewer identity")
	var kind string
	publish := &cobra.Command{Use: "publish [observation-id]", Short: "Publish a reviewed local observation as a Forge candidate", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		store, err := memory.Open(repository)
		if err != nil {
			return err
		}
		items, err := store.List()
		if err != nil {
			return err
		}
		var observation *memory.Observation
		for i := range items {
			if items[i].ID == args[0] {
				observation = &items[i]
				break
			}
		}
		if observation == nil {
			return fmt.Errorf("observation %q not found", args[0])
		}
		if observation.State != memory.Approved {
			return fmt.Errorf("only an explicitly approved observation can be published")
		}
		switch harnessdomain.KnowledgeKind(kind) {
		case harnessdomain.KnowledgeFact, harnessdomain.KnowledgeConvention, harnessdomain.KnowledgeBusinessRule, harnessdomain.KnowledgeDecision, harnessdomain.KnowledgeConstraint:
		default:
			return fmt.Errorf("--kind must classify the knowledge item")
		}
		item := harnessdomain.KnowledgeItem{ID: "observation-" + observation.ID, Kind: harnessdomain.KnowledgeKind(kind), Content: observation.Content, Origin: "local-observation:" + observation.ID + "; source:" + observation.Source, Review: harnessdomain.KnowledgeCandidate, Health: harnessdomain.KnowledgeUnknown}
		for _, e := range observation.Evidence {
			item.Evidence = append(item.Evidence, harnessdomain.KnowledgeEvidence{Path: e.Path, SHA256: e.SHA256, Quote: e.Quote})
		}
		path, err := harnessinfra.ImportKnowledgeCandidate(repository, item)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Published candidate %s at %s. Review it explicitly before sync.\n", item.ID, path)
		return err
	}}
	publish.Flags().StringVar(&repository, "repository", ".", "Repository with a Forge layout")
	publish.Flags().StringVar(&kind, "kind", "", "Knowledge type: fact, convention, business_rule, decision, constraint")
	root.AddCommand(capture, list, review, gc, publish)
	return root
}
