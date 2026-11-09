package encode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

// Sounds, for audio posts (roadmap row 137, docs/plans/audio-and-video.md):
// read as the kinds a browser records or a person has on their computer,
// and made into one file, AAC at 128 kbit/s in an M4A, with the picture a
// file carries (an album's cover) taken out as the post's cover.

// ErrNotASound is a file that has no sound to make an audio post of.
var ErrNotASound = errors.New("the file has no sound")

// AudioContainer says which kind of sound file a file is, from its first
// bytes, in ffmpeg's own names; "" for none it takes. M4A and WebM sound
// are the containers videos come in.
func AudioContainer(head []byte) string {
	switch {
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE":
		return "wav"
	case len(head) >= 4 && string(head[:4]) == "fLaC":
		return "flac"
	case len(head) >= 4 && string(head[:4]) == "OggS":
		return "ogg"
	case len(head) >= 3 && string(head[:3]) == "ID3":
		return "mp3"
	// An MPEG audio frame's sync: eleven bits set. Layer III is MP3; the
	// layer bits zero, ADTS, is a bare AAC stream.
	case len(head) >= 2 && head[0] == 0xFF && head[1]&0xE0 == 0xE0:
		if head[1]&0x06 == 0 {
			return "aac"
		}
		return "mp3"
	case len(head) >= 12 && string(head[4:8]) == "ftyp":
		return "mov"
	case len(head) >= 4 && bytes.Equal(head[:4], []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return "matroska"
	}
	return ""
}

// AudioContainerOf reads the first bytes of a file and says what it is.
func AudioContainerOf(path string) (string, error) {
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
	container := AudioContainer(head[:n])
	if container == "" {
		return "", ErrNotASound
	}
	return container, nil
}

// SoundInfo is what is known of a sound file once read.
type SoundInfo struct {
	Duration float64
	// It carries a picture: an album's cover.
	HasCover bool
}

type probedSound struct {
	Streams []struct {
		CodecType   string `json:"codec_type"`
		Disposition struct {
			AttachedPic int `json:"attached_pic"`
		} `json:"disposition"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// ParseSoundProbe reads what ffprobe said of a sound file: it has to have
// a sound, and be no longer than `maxSeconds`.
func ParseSoundProbe(output []byte, maxSeconds int) (*SoundInfo, error) {
	var p probedSound
	if err := json.Unmarshal(output, &p); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotASound, err)
	}
	info := &SoundInfo{}
	sound := false
	for _, stream := range p.Streams {
		switch {
		case stream.CodecType == "audio":
			sound = true
		case stream.CodecType == "video" && stream.Disposition.AttachedPic == 1:
			info.HasCover = true
		}
	}
	if !sound {
		return nil, ErrNotASound
	}
	duration, ok := seconds(p.Format.Duration)
	if !ok {
		return nil, ErrNotASound
	}
	info.Duration = duration
	if maxSeconds > 0 && duration > float64(maxSeconds)+1 {
		return nil, ErrTooLong
	}
	return info, nil
}

// ProbeSound reads a sound file.
func ProbeSound(ctx context.Context, path string, maxSeconds int) (*SoundInfo, string, error) {
	container, err := AudioContainerOf(path)
	if err != nil {
		return nil, "", err
	}
	args := append([]string{"-v", "error", "-print_format", "json", "-show_format", "-show_streams"},
		input(container, path)[:4]...)
	args = append(args, path)
	out, err := run(ctx, "ffprobe", args...)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrNotASound, err)
	}
	info, err := ParseSoundProbe(out, maxSeconds)
	return info, container, err
}

// EncodeSound makes the one file an audio post keeps: its first sound, as
// AAC at 128 kbit/s in an M4A, made to start playing before it has all
// arrived, and nothing else of the file.
func EncodeSound(ctx context.Context, container, in, out string, maxSeconds int) error {
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y"}
	args = append(args, input(container, in)...)
	args = append(args,
		"-map", "0:a:0",
		"-vn",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2",
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

// Cover takes out the picture a sound file carries, as a JPEG no wider
// than a poster is.
func Cover(ctx context.Context, container, in, out string) error {
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y"}
	args = append(args, input(container, in)...)
	args = append(args,
		"-map", "0:v:0",
		"-frames:v", "1",
		"-vf", "scale='min(1280,iw)':-2",
		"-c:v", "mjpeg",
		"-map_metadata", "-1",
		"-f", "image2", out,
	)
	_, err := run(ctx, "ffmpeg", args...)
	return err
}
