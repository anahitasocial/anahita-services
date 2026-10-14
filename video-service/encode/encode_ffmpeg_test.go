package encode

import (
	"context"
	"errors"
	"fmt"
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

		info, container, err := Probe(ctx, bare, 600, 30)
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

			made, kind, err := Probe(ctx, out, 600, 30)
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
		if _, _, err := Probe(ctx, path, 600, 30); !errors.Is(err, ErrNotAVideo) {
			t.Errorf("%s: %v", name, err)
		}
	}

	// Begins like an MP4 and is nothing of the kind.
	fake := filepath.Join(dir, "fake")
	_ = os.WriteFile(fake, append([]byte{0, 0, 0, 0x18}, []byte("ftypmp42 and then rubbish, a great deal of it")...), 0o644)
	if _, _, err := Probe(ctx, fake, 600, 30); !errors.Is(err, ErrNotAVideo) {
		t.Errorf("rubbish that begins like an MP4: %v", err)
	}

	// Sound alone is not a video.
	sound := filepath.Join(dir, "sound.mp4")
	if output, err := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "sine=duration=2", sound).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, output)
	}
	if _, _, err := Probe(ctx, sound, 600, 30); !errors.Is(err, ErrNotAVideo) {
		t.Errorf("sound alone: %v", err)
	}

	// Longer than is allowed.
	long := sample(t, dir, "long.mp4")
	if _, _, err := Probe(ctx, long, 1, 1); !errors.Is(err, ErrTooLong) {
		t.Errorf("three seconds where one is allowed: %v", err)
	}
}

// picture makes a moving (or still) picture with ffmpeg itself.
func picture(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	all := append([]string{"-nostdin", "-loglevel", "error", "-y"}, args...)
	all = append(all, out)
	if output, err := exec.Command("ffmpeg", all...).CombinedOutput(); err != nil {
		t.Fatalf("making %s: %v: %s", name, err, output)
	}
	return out
}

// pixel reads the colour at x, y of the first frame of a file.
func pixel(t *testing.T, path string, x, y int) []byte {
	t.Helper()
	out, err := exec.Command("ffmpeg", "-v", "error", "-i", path, "-frames:v", "1",
		"-vf", fmt.Sprintf("crop=2:2:%d:%d", x, y), "-f", "rawvideo", "-pix_fmt", "rgb24", "-").Output()
	if err != nil || len(out) < 3 {
		t.Fatalf("reading a pixel of %s: %v", path, err)
	}
	return out[:3]
}

func TestFfmpegMakesALoopOfAMovingPicture(t *testing.T) {
	ffmpegOrSkip(t)
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// An odd size, as GIFs are; and an animated PNG whose left half is
	// see-through.
	gif := picture(t, dir, "a.gif", "-f", "lavfi", "-i", "testsrc=size=321x241:rate=10:duration=2")
	apng := picture(t, dir, "a.png", "-f", "lavfi",
		"-i", "testsrc=s=200x150:d=2:r=5,format=rgba,geq=r=r(X\\,Y):g=g(X\\,Y):b=b(X\\,Y):a=if(lt(X\\,100)\\,0\\,255)",
		"-plays", "0", "-f", "apng")

	for name, c := range map[string]struct {
		path      string
		container string
		frames    int
		want      Target
	}{
		"a GIF":           {gif, GIF, 20, Target{Height: 240, Width: 320}},
		"an animated PNG": {apng, APNG, 10, Target{Height: 150, Width: 200}},
	} {
		bare := filepath.Join(dir, "upload")
		data, _ := os.ReadFile(c.path)
		_ = os.WriteFile(bare, data, 0o644)

		info, container, err := Probe(ctx, bare, 600, 30)
		if err != nil || container != c.container || !info.Animated || info.Frames != c.frames || info.Duration < 1.9 || info.Duration > 2.1 {
			t.Fatalf("%s: read as %q %+v %v", name, container, info, err)
		}

		targets := PlanAnimation(info)
		if len(targets) != 1 || targets[0] != c.want {
			t.Fatalf("%s: planned %v", name, targets)
		}
		out := filepath.Join(dir, "out.mp4")
		if err := Encode(ctx, container, bare, out, targets[0], 30); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		made, kind, err := Probe(ctx, out, 600, 30)
		if err != nil || kind != "mov" || made.Height != c.want.Height || made.Width != c.want.Width || made.HasAudio || made.Duration < 1.9 {
			t.Errorf("%s: made %q %+v %v", name, kind, made, err)
		}

		poster := filepath.Join(dir, "poster.jpg")
		if err := Poster(ctx, container, bare, poster, info); err != nil {
			t.Fatalf("%s: poster: %v", name, err)
		}
		if container == APNG {
			// What was see-through is white, in the clip and its poster.
			for _, file := range []string{out, poster} {
				if rgb := pixel(t, file, 10, 10); rgb[0] < 240 || rgb[1] < 240 || rgb[2] < 240 {
					t.Errorf("%s: see-through came out as %v in %s", name, rgb, filepath.Base(file))
				}
			}
		}
	}

	// Still pictures, and one too long.
	for name, c := range map[string]struct {
		path string
		want error
		max  int
	}{
		"a still GIF": {picture(t, dir, "still.gif", "-f", "lavfi", "-i", "testsrc=s=64x48", "-frames:v", "1"), ErrStill, 30},
		"a still PNG": {picture(t, dir, "still.png", "-f", "lavfi", "-i", "testsrc=s=64x48", "-frames:v", "1"), ErrStill, 30},
		// Four seconds where one is allowed, with a second's grace.
		"a long GIF": {picture(t, dir, "long.gif", "-f", "lavfi", "-i", "testsrc=s=64x48:rate=5:duration=4"), ErrTooLong, 1},
		"a tiny GIF": {picture(t, dir, "tiny.gif", "-f", "lavfi", "-i", "testsrc=s=8x8:rate=5:duration=1"), ErrNotAVideo, 30},
	} {
		if _, _, err := Probe(ctx, c.path, 600, c.max); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, wanted %v", name, err, c.want)
		}
	}
}
