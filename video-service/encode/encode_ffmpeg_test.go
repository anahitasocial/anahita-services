package encode

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// With the real ffmpeg. Skipped where there is none, which is anywhere but
// the worker's own image:
//
//	GOOS=linux go test -c -o encode.test ./encode
//	docker run --rm -v "$PWD/encode.test:/encode.test:ro" --entrypoint /encode.test \
//	  anahita-services-video-worker -test.run Ffmpeg -test.v
func ffmpegOrSkip(t *testing.T) {
	t.Helper()
	for _, program := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(program); err != nil {
			t.Skipf("%s is not installed", program)
		}
	}
}

// sample makes a few seconds of test picture and tone.
func sample(t *testing.T, dir, name string, extra ...string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	args := []string{"-nostdin", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=3:size=1920x1080:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-metadata", "title=secret title", "-metadata", "location=+49.2827-123.1207/",
		"-pix_fmt", "yuv420p"}
	args = append(args, extra...)
	args = append(args, out)
	if output, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("making a sample: %v: %s", err, output)
	}
	return out
}

func TestFfmpegMakesTheSizesOfARealVideo(t *testing.T) {
	ffmpegOrSkip(t)
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	for _, name := range []string{"source.mp4", "source.webm", "source.mov"} {
		source := sample(t, dir, name)
		// As it arrives: with no name to go by.
		bare := filepath.Join(dir, "upload")
		if err := os.Rename(source, bare); err != nil {
			t.Fatal(err)
		}

		info, container, err := Probe(ctx, bare, 600)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if info.Width != 1920 || info.Height != 1080 || info.Duration < 2.5 || info.Duration > 3.5 || !info.HasAudio {
			t.Fatalf("%s: read as %+v", name, info)
		}

		for _, target := range Plan(info, []int{1080, 720, 360}) {
			out := filepath.Join(dir, "out.mp4")
			if err := Encode(ctx, container, bare, out, target, 600); err != nil {
				t.Fatalf("%s at %d: %v", name, target.Height, err)
			}

			made, kind, err := Probe(ctx, out, 600)
			if err != nil || kind != "mov" {
				t.Fatalf("%s at %d: what was made is %q %v", name, target.Height, kind, err)
			}
			if made.Height != target.Height || made.Width != target.Width || !made.HasAudio || made.Duration < 2.5 {
				t.Errorf("%s at %d: made %+v, wanted %+v", name, target.Height, made, target)
			}

			// Nothing said in the upload is said in what is made of it.
			tags, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format_tags", "-of", "json", out).Output()
			if err != nil {
				t.Fatal(err)
			}
			for _, kept := range []string{"secret title", "49.2827"} {
				if containsBytes(tags, kept) {
					t.Errorf("%s at %d: %q was carried over: %s", name, target.Height, kept, tags)
				}
			}
		}

		poster := filepath.Join(dir, "poster.jpg")
		if err := Poster(ctx, container, bare, poster, info); err != nil {
			t.Fatalf("%s: poster: %v", name, err)
		}
		head := make([]byte, 3)
		file, _ := os.Open(poster)
		_, _ = file.Read(head)
		file.Close()
		if head[0] != 0xFF || head[1] != 0xD8 {
			t.Errorf("%s: the poster is not a JPEG", name)
		}
	}
}

func containsBytes(haystack []byte, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && (string(haystack) != "" && indexOf(string(haystack), needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestFfmpegIsNotGivenWhatIsNotAVideo(t *testing.T) {
	ffmpegOrSkip(t)
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	secret := filepath.Join(dir, "secret.txt")
	_ = os.WriteFile(secret, []byte("not to be read"), 0o644)

	for name, content := range map[string]string{
		"a playlist of local files": "#EXTM3U\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:1,\nfile://" + secret + "\n#EXT-X-ENDLIST\n",
		"a list to join":            "ffconcat version 1.0\nfile '" + secret + "'\n",
		"a page":                    "<html><body>hello</body></html>",
	} {
		path := filepath.Join(dir, "upload")
		_ = os.WriteFile(path, []byte(content), 0o644)
		if _, _, err := Probe(ctx, path, 600); !errors.Is(err, ErrNotAVideo) {
			t.Errorf("%s: %v", name, err)
		}
	}

	// Begins like an MP4 and is nothing of the kind.
	fake := filepath.Join(dir, "fake")
	_ = os.WriteFile(fake, append([]byte{0, 0, 0, 0x18}, []byte("ftypmp42 and then rubbish, a great deal of it")...), 0o644)
	if _, _, err := Probe(ctx, fake, 600); !errors.Is(err, ErrNotAVideo) {
		t.Errorf("rubbish that begins like an MP4: %v", err)
	}

	// Sound alone is not a video.
	sound := filepath.Join(dir, "sound.mp4")
	if output, err := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "sine=duration=2", sound).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, output)
	}
	if _, _, err := Probe(ctx, sound, 600); !errors.Is(err, ErrNotAVideo) {
		t.Errorf("sound alone: %v", err)
	}

	// Longer than is allowed.
	long := sample(t, dir, "long.mp4")
	if _, _, err := Probe(ctx, long, 1); !errors.Is(err, ErrTooLong) {
		t.Errorf("three seconds where one is allowed: %v", err)
	}
}
