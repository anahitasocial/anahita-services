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

	"anahita-libs/data/entities"
	"anahita-libs/videos"

	"video/constants"
	"video/data/repositories"
	"video/encode"
)

// Videos from other networks, copied here the first time somebody plays
// them (roadmap row 120, docs/plans/audio-and-video.md).
//
// The post says where its file is (videos.Elsewhere, in its meta). The
// worker fetches it from there with a client that reaches only public
// addresses, keeps one file of it, up to ElsewhereHeight, and puts that
// into the post's meta as a video posted here is, so that it plays the
// same way, from this site's storage. A file that already plays everywhere
// is kept as it is, in seconds; any other is encoded once. Its picture
// stays the post's picture: no poster is made.

// ElsewhereHeight is the one size a video from elsewhere is kept at, or its
// own when smaller.
const ElsewhereHeight = 720

// The reasons a copy from elsewhere is not made, beside the upload's own.
const (
	// The file is gone from its server, or the post is gone from here.
	ElsewhereGone = "gone"
	// Its server is one this site refuses.
	ElsewhereRefused = "refused"
	// Something here went wrong: tried again when asked again.
	ElsewhereFailedHere = "failed"
)

// ElsewhereNodes reads a post and writes its meta again.
type ElsewhereNodes interface {
	Read(id int64) (*entities.AnahitaNode, error)
	UpsertRemote(node *entities.AnahitaNode) (*entities.AnahitaNode, bool, error)
}

var errGone = errors.New("the file is no longer there")

// OnceElsewhere copies one video from elsewhere that is waiting, and says
// whether there was one.
func (w *Worker) OnceElsewhere() (bool, error) {
	if w.Elsewhere == nil || w.Nodes == nil {
		return false, nil
	}
	c, err := w.Elsewhere.Claim()
	if err != nil || c == nil {
		return false, err
	}
	log.Printf("from elsewhere %d: taken, attempt %d", c.NodeID, c.Attempts)
	w.copyElsewhere(c)
	return true, nil
}

func (w *Worker) copyElsewhere(c *repositories.Copy) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutFor(w.RemoteMaxSeconds, []int{ElsewhereHeight}))
	defer cancel()

	finish := func(status, reason string) {
		if err := w.Elsewhere.Finish(c.NodeID, status, reason); err != nil {
			log.Printf("from elsewhere %d: %v", c.NodeID, err)
		}
	}

	post, err := w.Nodes.Read(c.NodeID)
	if err != nil {
		log.Printf("from elsewhere %d: %v", c.NodeID, err)
		finish(repositories.ElsewhereFailed, ElsewhereFailedHere)
		return
	}
	if post == nil || post.ID == 0 {
		finish(repositories.ElsewhereFailed, ElsewhereGone)
		return
	}
	elsewhere := videos.ElsewhereFromMeta(post.Meta)
	if elsewhere == nil || elsewhere.Kind != videos.ElsewhereVideo {
		finish(repositories.ElsewhereFailed, ElsewhereGone)
		return
	}
	if w.Refused != nil && w.Refused(elsewhere.Host()) {
		finish(repositories.ElsewhereFailed, ElsewhereRefused)
		return
	}

	dir, err := os.MkdirTemp(w.Dir, "elsewhere-")
	if err != nil {
		log.Printf("from elsewhere %d: %v", c.NodeID, err)
		finish(repositories.ElsewhereFailed, ElsewhereFailedHere)
		return
	}
	defer os.RemoveAll(dir)

	video, err := w.makeElsewhere(ctx, post, elsewhere, dir)
	if err == nil {
		err = w.intoPost(post, video)
	}
	if err != nil {
		log.Printf("from elsewhere %d: %v", c.NodeID, err)
		w.remove(video.Keys())
		switch {
		case errors.Is(err, errGone):
			finish(repositories.ElsewhereFailed, ElsewhereGone)
		case errors.Is(err, errTooLarge):
			finish(repositories.ElsewhereFailed, constants.FailedTooLarge)
		case errors.Is(err, encode.ErrTooLong):
			finish(repositories.ElsewhereFailed, constants.FailedTooLong)
		case errors.Is(err, encode.ErrNotAVideo), errors.Is(err, encode.ErrStill):
			finish(repositories.ElsewhereFailed, constants.FailedNotAVideo)
		default:
			finish(repositories.ElsewhereFailed, ElsewhereFailedHere)
		}
		return
	}

	finish(repositories.ElsewhereReady, "")
	log.Printf("from elsewhere %d: ready, %d bytes", c.NodeID, video.Renditions[0].Bytes)
}

// makeElsewhere fetches the file and stores the one file kept of it.
func (w *Worker) makeElsewhere(ctx context.Context, post *entities.AnahitaNode, elsewhere *videos.Elsewhere, dir string) (*videos.Video, error) {
	video := &videos.Video{}

	source := filepath.Join(dir, "source")
	if err := w.fetchElsewhere(ctx, elsewhere.Source, source); err != nil {
		return video, err
	}
	info, container, err := w.Encoder.Probe(ctx, source, w.RemoteMaxSeconds, 0)
	if err != nil {
		return video, err
	}
	if info.Animated {
		return video, encode.ErrNotAVideo
	}

	out := filepath.Join(dir, "video.mp4")
	target := encode.Target{Height: info.Height, Width: info.Width}
	if encode.Playable(info, container, ElsewhereHeight) {
		err = w.Encoder.Remux(ctx, container, source, out, w.RemoteMaxSeconds)
	} else {
		targets := encode.Plan(info, []int{ElsewhereHeight})
		if len(targets) == 0 {
			return video, fmt.Errorf("no size to make")
		}
		target = targets[0]
		err = w.Encoder.Encode(ctx, container, source, out, target, w.RemoteMaxSeconds)
	}
	if err != nil {
		return video, err
	}

	key := videos.RenditionKey(videos.Folder(post.OwnerID, fmt.Sprintf("elsewhere-%d", post.ID)), target.Height)
	size, err := w.store(ctx, out, key, videos.MimeVideo)
	if err != nil {
		return video, err
	}
	video.Duration = info.Duration
	video.Width, video.Height = target.Width, target.Height
	video.Renditions = []videos.Rendition{{Height: target.Height, Width: target.Width, Key: key, Bytes: size}}
	return video, nil
}

// fetchElsewhere reads a file from another server into a file here, with
// the client that reaches only public addresses, and stops at the most a
// file from elsewhere may be.
func (w *Worker) fetchElsewhere(ctx context.Context, address, path string) error {
	if w.RemoteClient == nil {
		return errors.New("no client for other servers")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", w.UserAgent)
	res, err := w.RemoteClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
		return errGone
	case res.StatusCode != http.StatusOK:
		return fmt.Errorf("its server answered %d", res.StatusCode)
	}
	if w.RemoteMaxBytes > 0 && res.ContentLength > w.RemoteMaxBytes {
		return errTooLarge
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	written, err := io.Copy(file, io.LimitReader(res.Body, w.RemoteMaxBytes+1))
	if err != nil {
		return err
	}
	if w.RemoteMaxBytes > 0 && written > w.RemoteMaxBytes {
		return errTooLarge
	}
	return nil
}

// intoPost puts the copy into the post's meta, the rest of the post as it
// is.
func (w *Worker) intoPost(post *entities.AnahitaNode, video *videos.Video) error {
	meta, err := videos.IntoMeta(post.Meta, video)
	if err != nil {
		return err
	}
	_, _, err = w.Nodes.UpsertRemote(remoteAgain(post, meta))
	return err
}

// remoteAgain is a post from elsewhere as it is, with other meta: what the
// graph's UpsertRemote takes to change one.
func remoteAgain(post *entities.AnahitaNode, meta []byte) *entities.AnahitaNode {
	update := entities.NewAnahitaNode()
	update.Type, update.URI, update.OriginDomain = post.Type, post.URI, post.OriginDomain
	update.OwnerID, update.ParentID = post.OwnerID, post.ParentID
	update.Name, update.Body, update.Language, update.WebsiteURL = post.Name, post.Body, post.Language, post.WebsiteURL
	update.CreatedAt = post.CreatedAt
	update.Meta = meta
	return update
}

// TidyElsewhere removes the copies nobody has played for `days`, leaving
// the posts leading to where they are. Played again, they are copied
// again.
func (w *Worker) TidyElsewhere(days int, now time.Time) int {
	if w.Elsewhere == nil || w.Nodes == nil || days <= 0 {
		return 0
	}
	copies, err := w.Elsewhere.Unplayed(now.AddDate(0, 0, -days), 100)
	if err != nil {
		log.Printf("from elsewhere: tidying: %v", err)
		return 0
	}
	removed := 0
	for _, c := range copies {
		post, err := w.Nodes.Read(c.NodeID)
		if err != nil {
			continue
		}
		if post != nil && post.ID != 0 {
			if video := videos.FromMeta(post.Meta); video != nil {
				if _, _, err := w.Nodes.UpsertRemote(remoteAgain(post, videos.WithoutVideo(post.Meta))); err != nil {
					log.Printf("from elsewhere %d: %v", c.NodeID, err)
					continue
				}
				w.remove(video.Keys())
			}
		}
		if err := w.Elsewhere.Finish(c.NodeID, repositories.ElsewhereRemoved, ""); err == nil {
			removed++
		}
	}
	return removed
}
