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
//     filters). Only the two containers a video is uploaded in, and the two
//     kinds of moving picture (GIF and animated PNG), are let through, and
//     ffmpeg is told which one it is being given, so it never guesses.
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
	// ErrStill is a GIF or a PNG of one picture that does not move: a
	// photo, which is posted as one.
	ErrStill = errors.New("the picture does not move")
)

// The two kinds of moving picture taken, by ffmpeg's names for them. An
// animated WebP is not one: ffmpeg cannot yet read the frames of one.
const (
	GIF  = "gif"
	APNG = "apng"
)

// AnimationHeight is the most an animation is made tall. It is made once,
// at its own size up to this.
const AnimationHeight = 720

// IsAnimation says whether a container is one of the moving pictures, which
// are made into one silent clip that plays on a loop.
func IsAnimation(container string) bool {
	return container == GIF || container == APNG
}

// Info is what a video is, as far as making its sizes needs to know.
type Info struct {
	// As it is shown: a video filmed on a phone held upright is stored on
	// its side with a note to turn it, and these are after the turn.
	Width    int
	Height   int
	Duration float64
	HasAudio bool
	// A GIF or an animated PNG, and how many pictures it has.
	Animated bool
	Frames   int
}

// Target is one size to make.
type Target struct {
	Height int
	Width  int
}

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}

// Container says which of the kinds taken a file is, from its first
// bytes: "mov" for MP4 and QuickTime, "matroska" for WebM, "gif" and
// "apng" for the moving pictures. ffmpeg's own names for them. Whether a
// PNG moves is not in its first bytes; ContainerOf looks further.
func Container(head []byte) (string, error) {
	if len(head) >= 6 && (string(head[:6]) == "GIF87a" || string(head[:6]) == "GIF89a") {
		return GIF, nil
	}
	if len(head) >= 8 && bytes.Equal(head[:8], pngSignature) {
		return APNG, nil
	}
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
	container, err := Container(head[:n])
	if err != nil || container != APNG {
		return container, err
	}

	if _, err := file.Seek(int64(len(pngSignature)), io.SeekStart); err != nil {
		return "", err
	}
	moves, err := PNGMoves(file)
	if err != nil {
		return "", err
	}
	if !moves {
		return "", ErrStill
	}
	return APNG, nil
}

// PNGMoves reads the chunks of a PNG, from just after its signature, and
// says whether it is an animated one: one that says how it moves (acTL)
// before its picture begins (IDAT). ffmpeg's reader of animated PNGs will
// not open a still one, so this is how a still one is told apart, to be
// refused as a photo rather than as nothing at all.
func PNGMoves(r io.Reader) (bool, error) {
	header := make([]byte, 8)
	// Far more chunks than come before the picture in any real file.
	for i := 0; i < 64; i++ {
		if _, err := io.ReadFull(r, header); err != nil {
			return false, fmt.Errorf("%w: %v", ErrNotAVideo, err)
		}
		length := int64(header[0])<<24 | int64(header[1])<<16 | int64(header[2])<<8 | int64(header[3])
		switch string(header[4:8]) {
		case "acTL":
			return true, nil
		case "IDAT", "IEND":
			return false, nil
		}
		// The chunk's data, and its checksum.
		if _, err := io.CopyN(io.Discard, r, length+4); err != nil {
			return false, fmt.Errorf("%w: %v", ErrNotAVideo, err)
		}
	}
	return false, ErrNotAVideo
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
	// Asked for of a moving picture only: one for each of its pictures.
	Packets []struct {
		PTS      string `json:"pts_time"`
		Duration string `json:"duration_time"`
	} `json:"packets"`
}

func seconds(value string) (float64, bool) {
	n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || n <= 0 || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, false
	}
	return n, true
}

// ParseAnimation reads what ffprobe said of a GIF or an animated PNG,
// asked for with its pictures listed. One that has a single picture is a
// still. An animated PNG gives no length of its own, so it is where its
// last picture ends.
func ParseAnimation(output []byte, maxSeconds int) (*Info, error) {
	var p probed
	if err := json.Unmarshal(output, &p); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotAVideo, err)
	}

	info := &Info{Animated: true}
	for _, stream := range p.Streams {
		if stream.CodecType == "video" && stream.Width > 0 && stream.Height > 0 {
			info.Width, info.Height = stream.Width, stream.Height
			break
		}
	}
	// A file is served by a name of its height, of two digits at least;
	// and a picture less than ten pixels high is no picture to show.
	if info.Width < 10 || info.Height < 10 {
		return nil, ErrNotAVideo
	}

	info.Frames = len(p.Packets)
	if info.Frames < 2 {
		return nil, ErrStill
	}

	duration, ok := seconds(p.Format.Duration)
	if !ok {
		last := p.Packets[len(p.Packets)-1]
		at, _ := seconds(last.PTS)
		lasts, _ := seconds(last.Duration)
		duration = at + lasts
	}
	if duration <= 0 {
		return nil, ErrNotAVideo
	}
	info.Duration = duration
	if maxSeconds > 0 && duration > float64(maxSeconds)+1 {
		return nil, ErrTooLong
	}
	return info, nil
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

	duration, ok := seconds(p.Format.Duration)
	if !ok {
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

// PlanAnimation is the one size a moving picture is made in: its own, up to
// AnimationHeight tall, in numbers that divide by two.
func PlanAnimation(info *Info) []Target {
	height := info.Height
	width := info.Width
	if height > AnimationHeight {
		width = int(math.Round(float64(width) * float64(AnimationHeight) / float64(height)))
		height = AnimationHeight
	}
	return []Target{{Height: even(height), Width: even(width)}}
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

// Probe says what a file is, or that it is not a video. A moving picture
// is held to its own limit of length, animationSeconds.
func Probe(ctx context.Context, path string, maxSeconds, animationSeconds int) (*Info, string, error) {
	container, err := ContainerOf(path)
	if err != nil {
		return nil, "", err
	}

	if IsAnimation(container) {
		args := append([]string{"-v", "error", "-print_format", "json", "-select_streams", "v:0",
			"-show_entries", "stream=codec_type,width,height:format=duration:packet=pts_time,duration_time"},
			input(container, path)[:4]...)
		args = append(args, path)
		out, err := run(ctx, "ffprobe", args...)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", ErrNotAVideo, err)
		}
		info, err := ParseAnimation(out, animationSeconds)
		return info, container, err
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

// onWhite lays a picture that may be see-through on white, since an MP4
// and a JPEG have nothing see-through: it is drawn over a copy of itself
// filled white.
const onWhite = "format=rgba,split[fg][bg0];[bg0]drawbox=x=0:y=0:w=iw:h=ih:color=white:t=fill[bg];[bg][fg]overlay=format=auto"

// Encode makes one size of a video: H.264 and AAC in an MP4 a browser can
// start playing before it has all of it.
//
// A moving picture is made once, as a clip with no sound, on white, every
// picture kept for as long as it was shown (-fps_mode passthrough: a GIF's
// pictures need not be evenly spaced). The clip is played on a loop.
func Encode(ctx context.Context, container, in, out string, target Target, maxSeconds int) error {
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y"}
	args = append(args, input(container, in)...)
	if IsAnimation(container) {
		args = append(args,
			"-map", "0:v:0",
			// The height is made even as well: a GIF may be any size.
			"-vf", fmt.Sprintf("%s,scale=-2:trunc(min(%d\\,ih)/2)*2:flags=bicubic,format=yuv420p", onWhite, target.Height),
			"-fps_mode", "passthrough",
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-profile:v", "high",
			"-an",
			"-map_metadata", "-1", "-map_chapters", "-1", "-sn", "-dn",
			"-movflags", "+faststart",
		)
		if maxSeconds > 0 {
			args = append(args, "-t", strconv.Itoa(maxSeconds+1))
		}
		args = append(args, "-f", "mp4", out)
		_, err := run(ctx, "ffmpeg", args...)
		return err
	}
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
// in, or from the middle of one shorter than that. A moving picture's is
// its first, on white, which is what it showed before it moved.
func Poster(ctx context.Context, container, in, out string, info *Info) error {
	at := 1.0
	if info.Duration < 2 {
		at = info.Duration / 2
	}
	scale := "scale='min(1280,iw)':-2"
	if IsAnimation(container) {
		at = 0
		scale = onWhite + ",scale=min(1280\\,iw):-2"
	}

	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-ss", strconv.FormatFloat(at, 'f', 2, 64)}
	args = append(args, input(container, in)...)
	args = append(args,
		"-map", "0:v:0",
		"-frames:v", "1",
		"-vf", scale,
		"-map_metadata", "-1",
		"-q:v", "3",
		"-f", "image2", "-c:v", "mjpeg", out,
	)

	_, err := run(ctx, "ffmpeg", args...)
	return err
}
