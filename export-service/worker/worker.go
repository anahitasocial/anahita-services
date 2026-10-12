// Package worker makes the archives people have asked for: it takes each
// request in turn, gathers the person's data into a zip file, stores it,
// and marks it ready for them to fetch.
package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"anahita-libs/data/services"

	"export/archive"
	"export/data/repositories"
)

// The folder archives are kept under in storage, and what one is stored
// as. The file service holds both: see its uploads.go.
const (
	StoragePrefix = "exports/"
	ArchiveType   = "application/zip"
)

// KeyOf is where one export's archive is kept.
func KeyOf(export *repositories.Export) string {
	return fmt.Sprintf("%s%d/%s.zip", StoragePrefix, export.PersonID, export.ID)
}

type Remover interface {
	Delete(req *services.FileDeleteRequest) error
}

// Worker does the work on one export at a time.
type Worker struct {
	Exports   repositories.ExportsInterface
	Source    archive.Source
	Files     archive.Files
	Transfers services.FileTransfersInterface
	Remover   Remover
	WebURL    string
	SiteName  string
	// Where an archive is made before it is stored.
	Dir string
	// How long an archive is kept once it is ready.
	KeepFor time.Duration
	// How long making one may take.
	Timeout time.Duration
	Client  *http.Client
}

// Once looks for one export that is waiting and makes it. It says whether
// there was one.
func (w *Worker) Once() (bool, error) {
	export, err := w.Exports.Claim()
	if err != nil || export == nil {
		return false, err
	}

	log.Printf("export %s: taken, attempt %d", export.ID, export.Attempts)
	w.process(export)
	return true, nil
}

func (w *Worker) process(export *repositories.Export) {
	timeout := w.Timeout
	if timeout <= 0 {
		timeout = time.Hour
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	key := KeyOf(export)
	size, summary, err := w.make(ctx, export, key)
	if err != nil {
		w.failed(export, key, err)
		return
	}

	if err := w.Exports.ToReady(export.ID, key, size, w.KeepFor); err != nil {
		// Given up while it was being made: what was made is not kept.
		log.Printf("export %s: made, but %v", export.ID, err)
		w.remove(key)
		return
	}
	// Nothing of what is in it is ever logged: only that it is done, and
	// how much of it there was.
	log.Printf("export %s: ready, %d bytes, %d posts, %d files, %d files missing",
		export.ID, size, summary.Posts, summary.Files, summary.Missing)
}

func (w *Worker) failed(export *repositories.Export, key string, err error) {
	log.Printf("export %s: %v", export.ID, err)
	w.remove(key)

	// Given up by whoever asked: there is nothing to record.
	if errors.Is(err, archive.ErrStopped) {
		return
	}
	if export.Attempts < repositories.MaxAttempts {
		_ = w.Exports.Release(export.ID)
		return
	}
	if err := w.Exports.ToFailed(export.ID, "failed"); err != nil {
		log.Printf("export %s: %v", export.ID, err)
	}
}

// make writes the archive to a file of the worker's own and then stores
// it, and answers with its size.
func (w *Worker) make(ctx context.Context, export *repositories.Export, key string) (int64, *archive.Summary, error) {
	file, err := os.CreateTemp(w.Dir, "export-*.zip")
	if err != nil {
		return 0, nil, err
	}
	defer os.Remove(file.Name())
	defer file.Close()

	builder := &archive.Builder{
		Source:   w.Source,
		Files:    w.Files,
		WebURL:   w.WebURL,
		SiteName: w.SiteName,
		// Asked between each page of posts and each file: an export given
		// up is not gone on with.
		Stopped: func() bool {
			if ctx.Err() != nil {
				return true
			}
			current, err := w.Exports.Read(export.ID)
			return err == nil && (current == nil || current.Status != repositories.StatusBuilding)
		},
	}

	summary, err := builder.Write(ctx, file, export.PersonID)
	if err != nil {
		return 0, nil, err
	}

	size, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, nil, err
	}

	address, err := w.Transfers.StoreURL(key, ArchiveType)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, address, file)
	if err != nil {
		return 0, nil, err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", ArchiveType)

	client := w.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("storing the archive answered %d", res.StatusCode)
	}

	return size, summary, nil
}

func (w *Worker) remove(key string) {
	if key == "" {
		return
	}
	req := services.NewFileRequest().Delete
	req.Path = key
	if err := w.Remover.Delete(req); err != nil {
		log.Printf("could not remove %s: %v", key, err)
	}
}

// Tidy does what is nobody's request: deletes the archives that have been
// kept as long as they are, and gives up on exports that were tried as
// often as they will be.
func (w *Worker) Tidy() {
	past, err := w.Exports.PastKeeping()
	if err != nil {
		log.Printf("tidy: %v", err)
	}
	for _, export := range past {
		// The archive first: a row that says "expired" over a file that is
		// still there would leave it for ever.
		w.remove(export.PackageKey)
		if err := w.Exports.ToExpired(export.ID); err == nil {
			log.Printf("export %s: kept its time, deleted", export.ID)
		}
	}

	worn, err := w.Exports.WornOut()
	if err != nil {
		log.Printf("tidy: %v", err)
	}
	for _, id := range worn {
		if err := w.Exports.ToFailed(id, "failed"); err == nil {
			log.Printf("export %s: tried as often as it will be, given up", id)
		}
	}
}

// Run works until the context ends.
func (w *Worker) Run(ctx context.Context) {
	tidy := time.NewTicker(15 * time.Minute)
	defer tidy.Stop()
	w.Tidy()

	for {
		worked, err := w.Once()
		if err != nil {
			log.Printf("worker: %v", err)
		}
		if worked {
			continue
		}

		select {
		case <-ctx.Done():
			return
		case <-tidy.C:
			w.Tidy()
		case <-time.After(5 * time.Second):
		}
	}
}
