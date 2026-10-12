// Package encode turns a file somebody uploaded into the files a video
// post plays: it decides whether the file is a video at all, and makes
// every size of it, and its poster, with ffmpeg.
//
// THE FILE IS HOSTILE UNTIL SHOWN OTHERWISE. It came from a browser and
// nothing has looked at it. So:
//
//   - What it is is read from its first bytes, here, before ffmpeg is given
//     it. ffmpeg will open a great many kinds of file, and some of them are
//     lists of other files and addresses to fetch (playlists, scripts of
//     filters). Only the two containers a video is uploaded in are let
//     through, and ffmpeg is told which one it is being given, so it never
//     guesses.
//   - ffmpeg may open files and nothing else: no addresses.
//   - Nothing of the upload is kept. Every file made is encoded afresh,
//     with no metadata copied across, so what was hidden in the upload
//     stays in the upload, which is then deleted.
package encode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

var (
	// ErrNotAVideo is a file that is not one of the kinds taken, or has no
	// picture in it.
	ErrNotAVideo = errors.New("the file is not a video")
	// ErrTooLong is a video longer than is allowed.
	ErrTooLong = errors.New("the video is longer than is allowed")
)

// Info is what a video is, as far as making its sizes needs to know.
type Info struct {
	// As it is shown: a video filmed on a phone held upright is stored on
	// its side with a note to turn it, and these are after the turn.
	Width    int
	Height   int
	Duration float64
	HasAudio bool
}

// Target is one size to make.
type Target struct {
	Height int
	Width  int
}

// Container says which of the kinds taken a file is, from its first
// bytes: "mov" for MP4 and QuickTime, "matroska" for WebM. ffmpeg's own
// names for them.
func Container(head []byte) (string, error) {
	// ISO base media: a box length, then "ftyp".
	if len(head) >= 12 && string(head[4:8]) == "ftyp" {
		return "mov", nil
	}
	// EBML, which Matroska and WebM begin with.
	if len(head) >= 4 && bytes.Equal(head[:4], []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		return "matroska", nil
	}
	return "", ErrNotAVideo
}

// ContainerOf reads the first bytes of a file and says what it is.
func ContainerOf(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	head := make([]byte, 16)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", err
	}
	return Container(head[:n])
}

type probed struct {
	Streams []struct {
		CodecType   string `json:"codec_type"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		Disposition struct {
			AttachedPic int `json:"attached_pic"`
		} `json:"disposition"`
		Tags struct {
			Rotate string `json:"rotate"`
		} `json:"tags"`
		SideData []struct {
			Rotation json.Number `json:"rotation"`
		} `json:"side_data_list"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// ParseProbe reads what ffprobe said of a file.
func ParseProbe(output []byte, maxSeconds int) (*Info, error) {
	var p probed
	if err := json.Unmarshal(output, &p); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotAVideo, err)
	}

	info := &Info{}
	found := false
	for _, stream := range p.Streams {
		switch stream.CodecType {
		case "audio":
			info.HasAudio = true
		case "video":
			// A cover picture in a sound file is a "video" of one frame.
			if found || stream.Disposition.AttachedPic == 1 || stream.Width <= 0 || stream.Height <= 0 {
				continue
			}
			found = true
			info.Width, info.Height = stream.Width, stream.Height

			turn := 0.0
			if stream.Tags.Rotate != "" {
				turn, _ = strconv.ParseFloat(stream.Tags.Rotate, 64)
			}
			for _, side := range stream.SideData {
				if value, err := side.Rotation.Float64(); err == nil && value != 0 {
					turn = value
				}
			}
			if quarter := int(math.Abs(math.Round(turn/90))) % 2; quarter == 1 {
				info.Width, info.Height = info.Height, info.Width
			}
		}
	}
	if !found {
		return nil, ErrNotAVideo
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(p.Format.Duration), 64)
	if err != nil || duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return nil, ErrNotAVideo
	}
	info.Duration = duration
	// A little over is the container rounding, not a longer video.
	if maxSeconds > 0 && duration > float64(maxSeconds)+1 {
		return nil, ErrTooLong
	}

	return info, nil
}

func even(n int) int {
	if n%2 != 0 {
		n--
	}
	if n < 2 {
		n = 2
	}
	return n
}

// Plan decides the sizes to make, largest first: each height asked for
// that the video is at least as tall as. A video is never made larger than
// it is, so one smaller than every height asked for is made once, at its
// own size, and called by the smallest of them.
func Plan(info *Info, heights []int) []Target {
	wanted := append([]int(nil), heights...)
	sort.Sort(sort.Reverse(sort.IntSlice(wanted)))

	var targets []Target
	for _, height := range wanted {
		if height <= 0 || info.Height < height {
			continue
		}
		targets = append(targets, Target{
			Height: height,
			Width:  even(int(math.Round(float64(info.Width) * float64(height) / float64(info.Height)))),
		})
	}

	if len(targets) == 0 && len(wanted) > 0 {
		targets = append(targets, Target{
			Height: wanted[len(wanted)-1],
			Width:  even(info.Width),
		})
	}
	return targets
}

// What every call to ffmpeg and ffprobe begins with: files only, and the
// one kind of file it was found to be.
func input(container, path string) []string {
	return []string{
		"-protocol_whitelist", "file",
		"-f", container,
		"-i", path,
	}
}

func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// Nothing of this process's own is handed on: no credentials in the
	// environment of a program reading a stranger's file.
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}

	out, err := cmd.Output()
	if err != nil {
		tail := stderr.String()
		if len(tail) > 600 {
			tail = tail[len(tail)-600:]
		}
		return nil, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(tail))
	}
	return out, nil
}

// Probe says what a file is, or that it is not a video.
func Probe(ctx context.Context, path string, maxSeconds int) (*Info, string, error) {
	container, err := ContainerOf(path)
	if err != nil {
		return nil, "", err
	}

	args := append([]string{"-v", "error", "-print_format", "json", "-show_format", "-show_streams"},
		input(container, path)[:4]...)
	args = append(args, path)

	out, err := run(ctx, "ffprobe", args...)
	if err != nil {
		// It began like a video and ffprobe could make nothing of it.
		return nil, "", fmt.Errorf("%w: %v", ErrNotAVideo, err)
	}

	info, err := ParseProbe(out, maxSeconds)
	return info, container, err
}

// Encode makes one size of a video: H.264 and AAC in an MP4 a browser can
// start playing before it has all of it.
func Encode(ctx context.Context, container, in, out string, target Target, maxSeconds int) error {
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y"}
	args = append(args, input(container, in)...)
	args = append(args,
		"-map", "0:v:0",
		"-map", "0:a:0?",
		// Never larger than it is; a width that divides by two.
		"-vf", fmt.Sprintf("scale=-2:'min(%d,ih)':flags=bicubic,format=yuv420p", target.Height),
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-profile:v", "high",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2",
		// Nothing carried over from the upload but its picture and sound.
		"-map_metadata", "-1", "-map_chapters", "-1", "-sn", "-dn",
		"-movflags", "+faststart",
		"-max_muxing_queue_size", "1024",
	)
	if maxSeconds > 0 {
		args = append(args, "-t", strconv.Itoa(maxSeconds+1))
	}
	args = append(args, "-f", "mp4", out)

	_, err := run(ctx, "ffmpeg", args...)
	return err
}

// Poster makes the still shown before a video plays: a frame from a second
// in, or from the middle of one shorter than that.
func Poster(ctx context.Context, container, in, out string, info *Info) error {
	at := 1.0
	if info.Duration < 2 {
		at = info.Duration / 2
	}

	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-ss", strconv.FormatFloat(at, 'f', 2, 64)}
	args = append(args, input(container, in)...)
	args = append(args,
		"-map", "0:v:0",
		"-frames:v", "1",
		"-vf", "scale='min(1280,iw)':-2",
		"-map_metadata", "-1",
		"-q:v", "3",
		"-f", "image2", "-c:v", "mjpeg", out,
	)

	_, err := run(ctx, "ffmpeg", args...)
	return err
}
