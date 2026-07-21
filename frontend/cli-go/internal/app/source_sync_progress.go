package app

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sqlrs/cli/internal/remotesource"
)

type sourceSyncProgress struct {
	writer      io.Writer
	verbose     bool
	interactive bool
	mu          sync.Mutex
	label       string
	done        chan struct{}
	finished    chan struct{}
	once        sync.Once
}

func newSourceSyncProgress(writer io.Writer, verbose bool) remotesource.Progress {
	if writer == nil {
		return nil
	}
	interactive := false
	if file, ok := writer.(*os.File); ok {
		interactive = isTerminalWriterFn(file)
	}
	if !verbose && !interactive {
		return nil
	}
	p := &sourceSyncProgress{writer: writer, verbose: verbose, interactive: interactive}
	if interactive && !verbose {
		p.done = make(chan struct{})
		p.finished = make(chan struct{})
		go p.runSpinner()
	}
	return p
}

func (p *sourceSyncProgress) Update(event remotesource.ProgressEvent) {
	if p == nil {
		return
	}
	if p.verbose {
		if line := formatSourceSyncProgressLine(event); line != "" {
			fmt.Fprintln(p.writer, line)
		}
		return
	}
	if event.Stage == remotesource.ProgressStageComplete || event.Stage == remotesource.ProgressStageError {
		p.stop()
		return
	}
	p.mu.Lock()
	p.label = formatSourceSyncOperation(event)
	p.mu.Unlock()
}

func (p *sourceSyncProgress) runSpinner() {
	defer close(p.finished)
	timer := time.NewTimer(spinnerInitialDelay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-p.done:
		return
	}
	ticker := time.NewTicker(spinnerTickInterval)
	defer ticker.Stop()
	frames, index, width := []string{"-", "\\", "|", "/"}, 0, 1
	for {
		select {
		case <-p.done:
			clearLineOut(p.writer, width)
			return
		case <-ticker.C:
			p.mu.Lock()
			label := p.label
			p.mu.Unlock()
			if label == "" {
				continue
			}
			clearLineOut(p.writer, width)
			line := label + " " + frames[index]
			fmt.Fprint(p.writer, line)
			width = len(line)
			index = (index + 1) % len(frames)
		}
	}
}

func (p *sourceSyncProgress) stop() {
	if p.done == nil {
		return
	}
	p.once.Do(func() { close(p.done); <-p.finished })
}

func formatSourceSyncOperation(event remotesource.ProgressEvent) string {
	switch event.Stage {
	case remotesource.ProgressStageStart:
		return "source sync: starting"
	case remotesource.ProgressStageRound:
		return fmt.Sprintf("source sync: resolving round %d", event.Round)
	case remotesource.ProgressStageFileHashed:
		return "source sync: hashing " + event.Path
	case remotesource.ProgressStageDirectoryListed:
		return "source sync: listing " + event.Path
	case remotesource.ProgressStageUploadStart, remotesource.ProgressStageUploadBytes:
		return "source sync: uploading " + event.Path
	case remotesource.ProgressStageRetry:
		return fmt.Sprintf("source sync: retrying round %d", event.Round+1)
	default:
		return "source sync"
	}
}

func formatSourceSyncProgressLine(event remotesource.ProgressEvent) string {
	digest := event.Digest
	if digest != "" {
		digest = " sha256:" + digest
	}
	switch event.Stage {
	case remotesource.ProgressStageStart:
		return "source sync: started"
	case remotesource.ProgressStageRound:
		return fmt.Sprintf("source sync: round %d requested %d manifest entries and %d blobs", event.Round, event.ManifestEntries, event.Blobs)
	case remotesource.ProgressStageFileHashed:
		return fmt.Sprintf("source sync: hashed %s%s", event.Path, digest)
	case remotesource.ProgressStageDirectoryListed:
		return "source sync: listed " + event.Path
	case remotesource.ProgressStageUploadStart:
		return fmt.Sprintf("source sync: uploading %s%s (0/%d bytes)", event.Path, digest, event.TotalBytes)
	case remotesource.ProgressStageUploadBytes:
		return fmt.Sprintf("source sync: uploading %s%s (%d/%d bytes)", event.Path, digest, event.Bytes, event.TotalBytes)
	case remotesource.ProgressStageUploadComplete:
		return fmt.Sprintf("source sync: uploaded %s%s (%d bytes)", event.Path, digest, event.Bytes)
	case remotesource.ProgressStageRetry:
		return fmt.Sprintf("source sync: retry after round %d (+%d hashes, +%d listings, +%d blobs)", event.Round, event.FileHashes, event.DirectoryListings, event.UploadedBlobs)
	case remotesource.ProgressStageComplete:
		return fmt.Sprintf("source sync: complete (%d hashes, %d listings, %d blobs)", event.FileHashes, event.DirectoryListings, event.UploadedBlobs)
	case remotesource.ProgressStageError:
		return "source sync: failed: " + strings.TrimSpace(event.Error)
	default:
		return ""
	}
}

var _ remotesource.Progress = (*sourceSyncProgress)(nil)
