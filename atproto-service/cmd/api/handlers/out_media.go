package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/federation"
	"anahita-libs/images"
	"anahita-libs/linkpreviews"
	"anahita-libs/videos"
	"atproto/bsky"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// withVideo gives a post the video of a video post: a size no taller than
// 720 and no heavier than Bluesky is sent, read from this installation's
// storage and given to the account's server; Bluesky makes it ready to
// play once the post is written. A video too long or too heavy to send,
// or one Bluesky would not take, goes as a card that leads to it, with
// its poster.
func (h *Handlers) withVideo(ctx context.Context, post *bsky.OutPost, note *entities.AnahitaNode, upload uploader) error {
	video := videos.FromMeta(note.Meta)
	if video == nil {
		return nil
	}
	if file := video.Shared(bsky.VideoMax); file != nil && video.Duration <= bsky.VideoSecondsMax && h.Transfers != nil {
		content, err := h.fetched(ctx, file.Key, bsky.VideoMax)
		if err != nil {
			// Storage may be back in a moment: tried again.
			return err
		}
		blob, err := upload(content, videos.MimeVideo)
		if errors.Is(err, bsky.ErrExpired) || errors.Is(err, bsky.ErrDenied) {
			return err
		}
		if err == nil {
			post.WithVideo(blob, file.Width, file.Height)
			return nil
		}
		log.Printf("the video of post %d was not taken by Bluesky, and goes as a card: %v", note.ID, err)
	}

	title := strings.TrimSpace(note.Name)
	if title == "" {
		title = h.Site.Web(fmt.Sprintf("/videos/%d", note.ID))
	}
	var poster *bsky.OutBlob
	if video.Poster != "" && h.Keeper != nil && h.Keeper.Files != nil {
		if content, mimeType := h.stored(video.Poster); content != nil {
			poster, _ = upload(content, mimeType)
		}
	}
	post.WithCard(h.Site.Web(fmt.Sprintf("/videos/%d", note.ID)), title, "", poster)
	return nil
}

// fetched reads a file from this installation's storage, whole, at an
// address only the services reach. One heavier than max is refused.
func (h *Handlers) fetched(ctx context.Context, key string, max int64) ([]byte, error) {
	address, err := h.Transfers.FetchURL(key)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	client := h.Fetch
	if client == nil {
		client = &http.Client{Timeout: bsky.VideoTimeout}
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reading %s answered %d", key, res.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(res.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > max {
		return nil, federation.Permanent(fmt.Errorf("%s is heavier than %d bytes", key, max))
	}
	return content, nil
}

// uploader gives a file to the account's server.
type uploader func(content []byte, mimeType string) (*bsky.OutBlob, error)

// errPicturesNotThere is a photo post whose pictures are not listed yet:
// it is said to be made a moment before they are, and is tried again.
var errPicturesNotThere = errors.New("the post's pictures are not there yet")

// withPictures gives a post the pictures of a photo post: read from this
// installation's storage, made small enough for Bluesky to take, and
// given to the account's server; and whether it has more than Bluesky
// takes.
func (h *Handlers) withPictures(post *bsky.OutPost, note *entities.AnahitaNode, upload uploader) (bool, error) {
	if h.Keeper == nil || h.Keeper.PhotoFiles == nil || h.Keeper.Files == nil {
		return false, nil
	}

	var files []*entities.PhotoFile
	for waited := time.Duration(0); ; waited += time.Second {
		var err error
		if files, err = h.Keeper.PhotoFiles.BrowsePhotoFiles([]int64{note.ID}); err != nil {
			return false, err
		}
		if len(files) > 0 {
			break
		}
		if waited >= h.PicturesWait {
			return false, errPicturesNotThere
		}
		time.Sleep(time.Second)
	}

	for _, file := range files {
		if len(post.Embed.GetImages()) >= bsky.ImagesMax {
			break
		}
		sizes, err := note.PortraitsFor(file.Filename)
		if err != nil {
			continue
		}
		content, mimeType := h.stored(sizes[entities.PortraitSizeOriginal].Url)
		if content == nil {
			continue
		}
		blob, err := upload(content, mimeType)
		if errors.Is(err, bsky.ErrExpired) || errors.Is(err, bsky.ErrDenied) {
			return false, err
		}
		if err != nil {
			log.Printf("a picture of post %d was not taken by Bluesky: %v", note.ID, err)
			continue
		}
		post.AddImage(blob, file.AltText, file.Width, file.Height)
	}
	return len(files) > bsky.ImagesMax, nil
}

// coverPicture is a post's cover, given to the account's server for its
// card, or nil.
func (h *Handlers) coverPicture(note *entities.AnahitaNode, upload uploader) *bsky.OutBlob {
	if note.CoverFilename == "" || h.Keeper == nil || h.Keeper.Files == nil {
		return nil
	}
	covers, err := note.GetCovers()
	if err != nil {
		return nil
	}
	content, mimeType := h.stored(covers[entities.CoverSizeMedium].Url)
	if content == nil {
		return nil
	}
	blob, err := upload(content, mimeType)
	if err != nil {
		return nil
	}
	return blob
}

// audioCover is an audio post's cover, sent to Bluesky for its card, or nil.
func (h *Handlers) audioCover(note *entities.AnahitaNode, upload uploader) *bsky.OutBlob {
	audio := videos.AudioFromMeta(note.Meta)
	if audio == nil || audio.Cover == "" || h.Keeper == nil || h.Keeper.Files == nil {
		return nil
	}
	content, mimeType := h.stored(audio.Cover)
	if content == nil {
		return nil
	}
	blob, err := upload(content, mimeType)
	if err != nil {
		return nil
	}
	return blob
}

// articleOpening is what an article's card says under its title: its
// opening words, plain, near 300 characters at most.
func articleOpening(article *entities.AnahitaNode) string {
	words := strings.Join(strings.Fields(bsky.NewOutPost(article.Body, "", time.Time{}).Text), " ")
	if letters := []rune(words); len(letters) > 300 {
		cut := string(letters[:300])
		if at := strings.LastIndex(cut, " "); at > 150 {
			cut = cut[:at]
		}
		words = strings.TrimRight(cut, " ,;:.") + "…"
	}
	return words
}

// cardPicture is the picture of a link's card, given to the account's
// server, or nil: a card goes without one before a post goes late.
func (h *Handlers) cardPicture(key string, upload uploader) *bsky.OutBlob {
	if key == "" || h.Keeper == nil || h.Keeper.Files == nil {
		return nil
	}
	content, mimeType := h.stored(linkpreviews.StoragePrefix + key)
	if content == nil {
		return nil
	}
	blob, err := upload(content, mimeType)
	if err != nil {
		return nil
	}
	return blob
}

// stored reads a picture from this installation's storage, made smaller
// until it is one Bluesky takes. Nil when it is not there, or cannot be
// made small enough.
func (h *Handlers) stored(path string) ([]byte, string) {
	if path == "" {
		return nil, ""
	}
	read := services.NewFileRequest().Read
	read.Path = path
	file, err := h.Keeper.Files.Read(read)
	if err != nil || file == nil || len(file.Content) == 0 {
		return nil, ""
	}

	content := file.Content
	for _, width := range []int{0, 2048, 1600, 1280, 1024, 800, 640} {
		if width > 0 {
			if content, err = images.FitImage(file.Content, width); err != nil {
				return nil, ""
			}
		}
		if len(content) <= bsky.BlobMax {
			return content, file.Mimetype
		}
		// Still too heavy in the kind of file it is: a drawing or a
		// picture of a screen, kept without loss. As a photograph is
		// kept, it is a fraction of the weight.
		if lighter := asJPEG(content); lighter != nil && len(lighter) <= bsky.BlobMax {
			return lighter, "image/jpeg"
		}
	}
	return nil, ""
}

// asJPEG writes a picture again as a JPEG, on white where it could be
// seen through. Nil when it cannot be read.
func asJPEG(content []byte) []byte {
	decoded, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil
	}
	flat := image.NewRGBA(decoded.Bounds())
	draw.Draw(flat, flat.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), decoded, decoded.Bounds().Min, draw.Over)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, flat, &jpeg.Options{Quality: 85}); err != nil {
		return nil
	}
	return out.Bytes()
}
