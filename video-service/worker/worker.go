// Package worker makes posts of the videos people have uploaded: it takes
// each upload that is all there, checks it, makes its sizes and its poster,
// stores them, and only then makes the post.
package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	an_const "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/videos"

	"video/constants"
	"video/data/repositories"
	"video/encode"

	"github.com/gosimple/slug"
)

// Encoder is what is done to a file. ffmpeg in the worker; something
// quicker in a test.
type Encoder interface {
	Probe(ctx context.Context, path string, maxSeconds int) (*encode.Info, string, error)
	Encode(ctx context.Context, container, in, out string, target encode.Target, maxSeconds int) error
	Poster(ctx context.Context, container, in, out string, info *encode.Info) error
}

// FFmpeg is the Encoder that runs ffmpeg.
type FFmpeg struct{}

func (FFmpeg) Probe(ctx context.Context, path string, maxSeconds int) (*encode.Info, string, error) {
	return encode.Probe(ctx, path, maxSeconds)
}

func (FFmpeg) Encode(ctx context.Context, container, in, out string, target encode.Target, maxSeconds int) error {
	return encode.Encode(ctx, container, in, out, target, maxSeconds)
}

func (FFmpeg) Poster(ctx context.Context, container, in, out string, info *encode.Info) error {
	return encode.Poster(ctx, container, in, out, info)
}

// Poster is how the post is made, once there is something to show.
type Posts interface {
	Add(item *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error)
	Delete(id int64, viewer *entities.Viewer) error
}

type Owners interface {
	Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error)
}

type Remover interface {
	Delete(req *services.FileDeleteRequest) error
}

// Worker does the work on one upload at a time.
type Worker struct {
	Uploads   repositories.UploadsInterface
	Transfers services.FileTransfersInterface
	Files     Remover
	Posts     Posts
	Owners    Owners
	Encoder   Encoder
	// Where files are kept while they are worked on.
	Dir string
	// The sizes to make, by height, and the limits a video is held to.
	Heights    []int
	MaxBytes   int64
	MaxSeconds int
	// How long all the work on one video may take.
	Timeout time.Duration
	Client  *http.Client
}

// Once looks for one upload that is waiting and does it. It says whether
// there was one.
func (w *Worker) Once() (bool, error) {
	upload, err := w.Uploads.Claim()
	if err != nil || upload == nil {
		return false, err
	}

	log.Printf("video %s: taken, attempt %d", upload.ID, upload.Attempts)
	w.process(upload)
	return true, nil
}

// process does everything for one upload and records how it went. Nothing
// it does is left half done where anybody can see it: the post is made
// last, from files that are already stored.
func (w *Worker) process(upload *repositories.Upload) {
	timeout := w.Timeout
	if timeout <= 0 {
		timeout = TimeoutFor(w.MaxSeconds, w.Heights)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	dir, err := os.MkdirTemp(w.Dir, "video-")
	if err != nil {
		log.Printf("video %s: %v", upload.ID, err)
		_ = w.Uploads.Release(upload.ID)
		return
	}
	defer os.RemoveAll(dir)

	video, err := w.make(ctx, upload, dir)
	if err != nil {
		w.failed(upload, video, err)
		return
	}

	node, viewer, err := w.post(upload, video)
	if err != nil {
		w.failed(upload, video, err)
		return
	}

	if err := w.Uploads.ToReady(upload.ID, node.ID); err != nil {
		// Given up in the moment the post was being made: taken back, with
		// its files.
		log.Printf("video %s: made, but %v", upload.ID, err)
		if err := w.Posts.Delete(node.ID, viewer); err != nil {
			log.Printf("video %s: post %d could not be taken back: %v", upload.ID, node.ID, err)
		}
		w.remove(video.Keys())
		w.remove([]string{upload.SourceKey})
		return
	}

	w.remove([]string{upload.SourceKey})
	log.Printf("video %s: ready as post %d", upload.ID, node.ID)
}

// TimeoutFor is how long the work on one video may take, for the longest
// video allowed and the sizes it is made in.
//
// Encoding takes about as long as there are pixels to make: 720p is the
// measure, at something like half again the time the video plays for on a
// small machine, 1080p is a little over twice that and 4K nine times. Never
// less than half an hour, which is room for fetching and storing as well.
func TimeoutFor(maxSeconds int, heights []int) time.Duration {
	weight := 0.0
	for _, height := range heights {
		scale := float64(height) / 720
		weight += scale * scale
	}
	if weight < 1 {
		weight = 1
	}

	timeout := time.Duration(float64(maxSeconds) * 1.5 * weight * float64(time.Second))
	if timeout < 30*time.Minute {
		timeout = 30 * time.Minute
	}
	return timeout
}

// failed records why a video was not made. What is wrong with the file
// itself is final. Anything else may be the store or the network, and is
// tried again until its attempts run out.
func (w *Worker) failed(upload *repositories.Upload, video *videos.Video, err error) {
	log.Printf("video %s: %v", upload.ID, err)
	w.remove(video.Keys())

	// Given up by its uploader while it was being made: nothing to record.
	if errors.Is(err, repositories.ErrNotInState) {
		w.remove([]string{upload.SourceKey})
		return
	}

	reason := ""
	switch {
	case errors.Is(err, encode.ErrNotAVideo):
		reason = constants.FailedNotAVideo
	case errors.Is(err, encode.ErrTooLong):
		reason = constants.FailedTooLong
	case errors.Is(err, errTooLarge):
		reason = constants.FailedTooLarge
	case upload.Attempts >= constants.MaxAttempts:
		reason = constants.FailedEncoding
	}

	if reason == "" {
		_ = w.Uploads.Release(upload.ID)
		return
	}
	if err := w.Uploads.ToFailed(upload.ID, reason); err != nil {
		log.Printf("video %s: %v", upload.ID, err)
	}
	w.remove([]string{upload.SourceKey})
}

var errTooLarge = errors.New("the file is larger than is allowed")

// make fetches the upload and stores every file made of it. It returns
// what was stored, as far as it got, for clearing up after a failure.
func (w *Worker) make(ctx context.Context, upload *repositories.Upload, dir string) (*videos.Video, error) {
	video := &videos.Video{}

	source := filepath.Join(dir, "source")
	if err := w.fetch(ctx, upload.SourceKey, source); err != nil {
		return video, err
	}

	info, container, err := w.Encoder.Probe(ctx, source, w.MaxSeconds)
	if err != nil {
		return video, err
	}

	targets := encode.Plan(info, w.Heights)
	if len(targets) == 0 {
		return video, fmt.Errorf("no sizes to make")
	}

	folder := videos.Folder(upload.OwnerID, upload.ID)
	video.Duration = info.Duration

	for i, target := range targets {
		out := filepath.Join(dir, fmt.Sprintf("%d.mp4", target.Height))
		if err := w.Encoder.Encode(ctx, container, source, out, target, w.MaxSeconds); err != nil {
			return video, err
		}

		key := videos.RenditionKey(folder, target.Height)
		size, err := w.store(ctx, out, key, videos.MimeVideo)
		if err != nil {
			return video, err
		}
		video.Renditions = append(video.Renditions, videos.Rendition{
			Height: target.Height,
			Width:  target.Width,
			Key:    key,
			Bytes:  size,
		})
		if i == 0 {
			video.Width, video.Height = target.Width, target.Height
		}
		_ = os.Remove(out)
	}

	poster := filepath.Join(dir, "poster.jpg")
	if err := w.Encoder.Poster(ctx, container, source, poster, info); err != nil {
		// A video with no still is still a video.
		log.Printf("video %s: no poster: %v", upload.ID, err)
		return video, nil
	}
	key := videos.PosterKey(folder)
	if _, err := w.store(ctx, poster, key, videos.MimePoster); err != nil {
		return video, err
	}
	video.Poster = key

	return video, nil
}

// post makes the post, as the person who uploaded the video, on the
// profile they chose, with everything they said about it then.
func (w *Worker) post(upload *repositories.Upload, video *videos.Video) (*entities.AnahitaNode, *entities.Viewer, error) {
	// Given up while it was being made: no post.
	current, err := w.Uploads.Read(upload.ID)
	if err != nil {
		return nil, nil, err
	}
	if current == nil || current.Status != videos.StatusProcessing {
		return nil, nil, repositories.ErrNotInState
	}

	viewer := entities.NewViewer()
	viewer.ID = upload.PersonID
	viewer.Authenticated = true
	viewer.Enabled = true
	viewer.PersonType = an_const.PersonTypeRegistered

	owner, err := w.Owners.Read(upload.OwnerID, viewer)
	if err != nil || owner == nil || owner.ID == 0 {
		return nil, nil, fmt.Errorf("the profile it was posted on is not there: %v", err)
	}

	medium := entities.NewAnahitaNode()
	medium.Type = constants.NodeMediumVideoType
	medium.Service = constants.VideoService
	medium.OwnerID = owner.ID
	medium.OwnerType = owner.Type
	medium.CreatedBy = upload.PersonID
	medium.ModifiedBy = upload.PersonID
	medium.Name = upload.Name
	medium.Alias = slug.Make(upload.Name)
	medium.Body = upload.Body
	medium.Access = upload.Access
	medium.Language = upload.Language
	medium.QuotePolicy = upload.QuotePolicy
	medium.Enabled = true
	medium.CommentStatus = true
	if upload.ReplyAccess != "" {
		medium.ReplyAccess = upload.ReplyAccess
		medium.CommentStatus = upload.ReplyAccess != an_const.ReplyAccessNobody
	}
	medium.Mimetype = videos.MimeVideo

	medium.Meta, err = videos.IntoMeta(nil, video)
	if err != nil {
		return nil, nil, err
	}

	node, err := w.Posts.Add(medium, viewer)
	if err != nil || node == nil {
		return nil, nil, fmt.Errorf("the post was not made: %v", err)
	}
	return node, viewer, nil
}

func (w *Worker) client() *http.Client {
	if w.Client != nil {
		return w.Client
	}
	return http.DefaultClient
}

// fetch reads the upload from storage into a file, and stops at the most a
// video may be: what was checked when the upload was finished is checked
// again where it is read.
func (w *Worker) fetch(ctx context.Context, key, path string) error {
	address, err := w.Transfers.FetchURL(key)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	res, err := w.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("reading the upload answered %d", res.StatusCode)
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	written, err := io.Copy(file, io.LimitReader(res.Body, w.MaxBytes+1))
	if err != nil {
		return err
	}
	if w.MaxBytes > 0 && written > w.MaxBytes {
		return errTooLarge
	}
	return nil
}

// store writes one of the worker's files to storage and returns its size.
func (w *Worker) store(ctx context.Context, path, key, contentType string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if stat.Size() == 0 {
		return 0, fmt.Errorf("nothing was made for %s", key)
	}

	address, err := w.Transfers.StoreURL(key, contentType)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, address, file)
	if err != nil {
		return 0, err
	}
	req.ContentLength = stat.Size()
	req.Header.Set("Content-Type", contentType)

	res, err := w.client().Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("storing %s answered %d", key, res.StatusCode)
	}

	return stat.Size(), nil
}

func (w *Worker) remove(keys []string) {
	for _, key := range keys {
		req := services.NewFileRequest().Delete
		req.Path = key
		if err := w.Files.Delete(req); err != nil {
			log.Printf("could not remove %s: %v", key, err)
		}
	}
}

// Tidy does what is nobody's request: gives up on uploads that were begun
// and never finished, and on videos that were tried as often as they will
// be, and forgets the records of what was settled long ago.
func (w *Worker) Tidy() {
	abandoned, err := w.Uploads.Abandoned()
	if err != nil {
		log.Printf("tidy: %v", err)
	}
	for _, upload := range abandoned {
		if err := w.Uploads.ToCancelled(upload.ID); err != nil {
			continue
		}
		if upload.StorageUploadID != "" {
			if err := w.Transfers.UploadAbort(upload.SourceKey, upload.StorageUploadID); err != nil {
				log.Printf("tidy: %v", err)
			}
		}
		log.Printf("video %s: never finished, given up", upload.ID)
	}

	worn, err := w.Uploads.WornOut()
	if err != nil {
		log.Printf("tidy: %v", err)
	}
	for _, id := range worn {
		upload, err := w.Uploads.Read(id)
		if err != nil || upload == nil {
			continue
		}
		if err := w.Uploads.ToFailed(id, constants.FailedEncoding); err == nil {
			w.remove([]string{upload.SourceKey})
			log.Printf("video %s: tried %d times, given up", id, upload.Attempts)
		}
	}

	if err := w.Uploads.Forget(time.Now().UTC().Add(-30 * 24 * time.Hour)); err != nil {
		log.Printf("tidy: %v", err)
	}
}

// Run works until the context ends: a video whenever one is waiting, and
// tidying now and then.
func (w *Worker) Run(ctx context.Context) {
	tidy := time.NewTicker(10 * time.Minute)
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
