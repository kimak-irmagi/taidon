// Package remotesource implements the CLI side of the remote source-sync
// protocol documented in docs/architecture/remote-source-input-sync-flow.md.
package remotesource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/inputset"
)

const DefaultMaxRounds = 8

type Uploader interface {
	PutSourceBlob(context.Context, string, io.Reader) error
}

type Options struct {
	Enabled       bool
	MaxRounds     int
	WorkspaceRoot string
	WorkDir       string
	WorkspaceID   string
	FileSystem    inputset.FileSystem
	Uploader      Uploader
	Progress      io.Writer
}

// Execute sends a prepare-shaped request through the remote source-sync loop.
// The server remains authoritative: the client only adds manifest entries and
// uploads blobs named by a recoverable source_inputs_missing response.
func Execute[T any](ctx context.Context, opts Options, req client.PrepareJobRequest, execute func(context.Context, client.PrepareJobRequest) (T, error)) (T, error) {
	if !opts.Enabled {
		return execute(ctx, req)
	}
	state := newRoundState(opts)
	maxRounds := opts.MaxRounds
	if maxRounds <= 0 {
		maxRounds = DefaultMaxRounds
	}

	req.SourceManifest = state.manifest()
	for round := 1; ; round++ {
		result, err := execute(ctx, req)
		if err == nil {
			return result, nil
		}

		var missing *client.SourceInputsMissingError
		if !errors.As(err, &missing) {
			var zero T
			return zero, err
		}
		if round > maxRounds {
			var zero T
			return zero, fmt.Errorf("source sync reached max rounds (%d): %w", maxRounds, err)
		}

		stats, changed, applyErr := state.applyMissing(ctx, missing.Response)
		if applyErr != nil {
			var zero T
			return zero, applyErr
		}
		state.report("source sync: round %d, requested %d manifest entries and %d blobs\n", round, len(missing.Response.MissingManifestEntries), len(missing.Response.MissingBlobs))
		if stats.FileHashes > 0 || stats.DirectoryListings > 0 {
			state.report("source sync: added %d file hashes and %d directory listings\n", stats.FileHashes, stats.DirectoryListings)
		}
		if stats.UploadedBlobs > 0 {
			state.report("source sync: uploaded %d blobs\n", stats.UploadedBlobs)
		}
		if !changed {
			var zero T
			return zero, fmt.Errorf("source sync made no progress after round %d: %w", round, err)
		}
		req.SourceManifest = state.manifest()
	}
}

type roundState struct {
	root        string
	workDir     string
	rootID      string
	fs          inputset.FileSystem
	uploader    Uploader
	progress    io.Writer
	files       map[string]string
	directories map[string]client.SourceDirectoryListing
	uploaded    map[string]struct{}
}

type applyStats struct {
	FileHashes        int
	DirectoryListings int
	UploadedBlobs     int
}

func newRoundState(opts Options) *roundState {
	root := strings.TrimSpace(opts.WorkspaceRoot)
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err == nil {
		root = absRoot
	}
	root = filepath.Clean(root)
	workDir := strings.TrimSpace(opts.WorkDir)
	if workDir == "" {
		workDir = root
	}
	if absWorkDir, err := filepath.Abs(workDir); err == nil {
		workDir = absWorkDir
	}
	workDir = filepath.Clean(workDir)
	rootID := strings.TrimSpace(opts.WorkspaceID)
	if rootID == "" {
		rootID = filepath.Base(root)
	}
	if rootID == "" || rootID == "." {
		rootID = "default"
	}
	sourceFS := opts.FileSystem
	if sourceFS == nil {
		sourceFS = inputset.OSFileSystem{}
	}
	progress := opts.Progress
	if progress == nil {
		progress = io.Discard
	}
	return &roundState{
		root:        root,
		workDir:     workDir,
		rootID:      rootID,
		fs:          sourceFS,
		uploader:    opts.Uploader,
		progress:    progress,
		files:       map[string]string{},
		directories: map[string]client.SourceDirectoryListing{},
		uploaded:    map[string]struct{}{},
	}
}

func (s *roundState) manifest() *client.SourceManifest {
	manifest := &client.SourceManifest{
		WorkspaceRef: &client.SourceWorkspaceRef{
			RootID:   s.rootID,
			RootPath: s.root,
			WorkDir:  s.workDir,
		},
	}
	if len(s.files) > 0 {
		manifest.Files = make(map[string]string, len(s.files))
		for key, value := range s.files {
			manifest.Files[key] = value
		}
	}
	if len(s.directories) > 0 {
		manifest.Directories = make(map[string]client.SourceDirectoryListing, len(s.directories))
		for key, value := range s.directories {
			entries := append([]client.SourceDirectoryEntry(nil), value.Entries...)
			manifest.Directories[key] = client.SourceDirectoryListing{Entries: entries, Complete: value.Complete}
		}
	}
	return manifest
}

func (s *roundState) applyMissing(ctx context.Context, missing client.SourceInputsMissingErrorResponse) (applyStats, bool, error) {
	stats := applyStats{}
	changed := false

	entries := append([]client.SourceMissingManifestEntry(nil), missing.MissingManifestEntries...)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Path == entries[j].Path {
			return entries[i].Kind < entries[j].Kind
		}
		return entries[i].Path < entries[j].Path
	})
	for _, entry := range entries {
		switch entry.Kind {
		case "file_hash":
			added, err := s.addFileHash(entry.Path)
			if err != nil {
				return stats, changed, err
			}
			if added {
				stats.FileHashes++
				changed = true
			}
		case "directory_listing":
			added, err := s.addDirectoryListing(entry.Path)
			if err != nil {
				return stats, changed, err
			}
			if added {
				stats.DirectoryListings++
				changed = true
			}
		default:
			return stats, changed, fmt.Errorf("unsupported source manifest entry kind %q for %s", entry.Kind, entry.Path)
		}
	}

	blobs := append([]client.SourceMissingBlob(nil), missing.MissingBlobs...)
	sort.Slice(blobs, func(i, j int) bool {
		if blobs[i].Hash == blobs[j].Hash {
			return blobs[i].Path < blobs[j].Path
		}
		return blobs[i].Hash < blobs[j].Hash
	})
	for _, blob := range blobs {
		uploaded, err := s.uploadBlob(ctx, blob)
		if err != nil {
			return stats, changed, err
		}
		if uploaded {
			stats.UploadedBlobs++
			changed = true
		}
	}

	return stats, changed, nil
}

func (s *roundState) addFileHash(manifestPath string) (bool, error) {
	cleaned, absPath, err := s.resolveManifestPath(manifestPath)
	if err != nil {
		return false, err
	}
	hash, _, err := s.readFileHash(absPath)
	if err != nil {
		return false, fmt.Errorf("hash source file %s: %w", cleaned, err)
	}
	if previous, ok := s.files[cleaned]; ok {
		if previous != hash {
			return false, fmt.Errorf("source file changed during sync: %s", cleaned)
		}
		return false, nil
	}
	s.files[cleaned] = hash
	return true, nil
}

func (s *roundState) addDirectoryListing(manifestPath string) (bool, error) {
	cleaned, absPath, err := s.resolveManifestPath(manifestPath)
	if err != nil {
		return false, err
	}
	entries, err := s.fs.ReadDir(absPath)
	if err != nil {
		return false, fmt.Errorf("list source directory %s: %w", cleaned, err)
	}
	listing := client.SourceDirectoryListing{
		Entries:  make([]client.SourceDirectoryEntry, 0, len(entries)),
		Complete: true,
	}
	for _, entry := range entries {
		kind := "file"
		if entry.IsDir() {
			kind = "directory"
		}
		listing.Entries = append(listing.Entries, client.SourceDirectoryEntry{
			Name: entry.Name(),
			Kind: kind,
		})
	}
	sort.Slice(listing.Entries, func(i, j int) bool {
		return listing.Entries[i].Name < listing.Entries[j].Name
	})
	if previous, ok := s.directories[cleaned]; ok {
		if !sameDirectoryListing(previous, listing) {
			return false, fmt.Errorf("source directory changed during sync: %s", cleaned)
		}
		return false, nil
	}
	s.directories[cleaned] = listing
	return true, nil
}

func (s *roundState) uploadBlob(ctx context.Context, blob client.SourceMissingBlob) (bool, error) {
	if s.uploader == nil {
		return false, fmt.Errorf("source blob upload is not configured")
	}
	expected := strings.TrimSpace(blob.Hash)
	digest, ok := strings.CutPrefix(expected, "sha256:")
	if !ok || !isLowerHexDigest(digest) {
		return false, fmt.Errorf("invalid source blob hash for %s: %s", blob.Path, blob.Hash)
	}
	cleaned, absPath, err := s.resolveManifestPath(blob.Path)
	if err != nil {
		return false, err
	}
	actual, content, err := s.readFileHash(absPath)
	if err != nil {
		return false, fmt.Errorf("read source blob %s: %w", cleaned, err)
	}
	if actual != expected {
		return false, fmt.Errorf("source blob hash mismatch for %s: expected %s, got %s", cleaned, expected, actual)
	}
	if previous, ok := s.files[cleaned]; ok && previous != actual {
		return false, fmt.Errorf("source file changed during sync: %s", cleaned)
	}
	changed := false
	if _, ok := s.files[cleaned]; !ok {
		s.files[cleaned] = actual
		changed = true
	}
	if _, ok := s.uploaded[digest]; ok {
		return changed, nil
	}
	if err := s.uploader.PutSourceBlob(ctx, digest, bytes.NewReader(content)); err != nil {
		return false, err
	}
	s.uploaded[digest] = struct{}{}
	return true, nil
}

func (s *roundState) resolveManifestPath(value string) (string, string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", "", fmt.Errorf("source manifest path is empty")
	}
	if strings.Contains(raw, "\\") {
		return "", "", fmt.Errorf("source manifest path must use slash separators: %s", value)
	}
	cleaned := path.Clean(raw)
	if path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") || driveQualified(cleaned) {
		return "", "", fmt.Errorf("source manifest path escapes workspace: %s", value)
	}
	absPath := filepath.Join(s.root, filepath.FromSlash(cleaned))
	canonicalRoot := inputset.CanonicalizeBoundaryPath(s.root)
	canonicalPath := inputset.CanonicalizeBoundaryPath(absPath)
	if !inputset.IsWithin(canonicalRoot, canonicalPath) {
		return "", "", fmt.Errorf("source manifest path escapes workspace: %s", value)
	}
	return cleaned, absPath, nil
}

func (s *roundState) readFileHash(absPath string) (string, []byte, error) {
	info, err := s.fs.Stat(absPath)
	if err != nil {
		return "", nil, err
	}
	if info.IsDir() {
		return "", nil, fs.ErrInvalid
	}
	content, err := s.fs.ReadFile(absPath)
	if err != nil {
		return "", nil, err
	}
	return "sha256:" + inputset.HashContent(content), content, nil
}

func (s *roundState) report(format string, args ...any) {
	if s.progress == io.Discard {
		return
	}
	fmt.Fprintf(s.progress, format, args...)
}

func sameDirectoryListing(left, right client.SourceDirectoryListing) bool {
	if left.Complete != right.Complete || len(left.Entries) != len(right.Entries) {
		return false
	}
	for i := range left.Entries {
		if left.Entries[i] != right.Entries[i] {
			return false
		}
	}
	return true
}

func isLowerHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}

func driveQualified(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	first := value[0]
	return (first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z')
}
