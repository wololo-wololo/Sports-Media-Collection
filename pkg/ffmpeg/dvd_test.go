package ffmpeg

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sparse creates a file of the given size without using disk space.
func sparse(t *testing.T, path string, size int64) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(size))
	require.NoError(t, f.Close())
}

func TestDVDTitles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "F1 1999 Nürburgring RTL", "VIDEO_TS")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	p := func(name string) string { return filepath.Join(dir, name) }

	sparse(t, p("VIDEO_TS.VOB"), 1<<20)
	sparse(t, p("VTS_01_0.VOB"), 2<<20)
	sparse(t, p("VTS_01_1.VOB"), 1<<30)
	sparse(t, p("VTS_01_2.VOB"), 1<<30)
	sparse(t, p("VTS_01_3.VOB"), 300<<20)
	sparse(t, p("VTS_02_1.VOB"), 20<<20) // short extra
	sparse(t, p("vts_03_1.vob"), 80<<20) // lower case, bonus programme
	sparse(t, p("vts_03_2.vob"), 80<<20)
	sparse(t, p("VTS_04_1.VOB"), 200<<20) // part 2 missing
	sparse(t, p("VTS_04_3.VOB"), 200<<20)

	assert.Equal(t, []string{p("VTS_01_1.VOB"), p("VTS_01_2.VOB"), p("VTS_01_3.VOB")}, DVDTitleParts(p("VTS_01_1.VOB")))
	assert.Nil(t, DVDTitleParts(p("VTS_01_2.VOB")), "only the first part stands for the title")
	assert.Nil(t, DVDTitleParts(p("VTS_01_0.VOB")), "menus are not titles")
	assert.Equal(t, []string{p("VTS_04_1.VOB")}, DVDTitleParts(p("VTS_04_1.VOB")), "parts stop at a gap")

	assert.True(t, IsDVDTitle(p("VTS_01_1.VOB")))
	assert.False(t, IsDVDTitle(p("VTS_01_2.VOB")))
	assert.False(t, IsDVDTitle(p("VTS_01_0.VOB")))
	assert.False(t, IsDVDTitle(p("VIDEO_TS.VOB")))
	assert.False(t, IsDVDTitle(p("VTS_02_1.VOB")), "extras below the size threshold")
	assert.True(t, IsDVDTitle(p("vts_03_1.vob")), "two parts together pass the threshold")
	assert.True(t, IsDVDTitle(p("VTS_04_1.VOB")))

	for _, name := range []string{"VIDEO_TS.VOB", "VTS_01_0.VOB", "VTS_01_2.VOB", "vts_03_1.vob"} {
		assert.True(t, IsDVDVob(p(name)), name)
	}
	assert.False(t, IsDVDVob("/videos/race.vob"), "loose VOB files follow the extension list")
	assert.False(t, IsDVDTitle("/videos/race.mkv"))

	assert.Equal(t, "concat:"+strings.Join([]string{p("VTS_01_1.VOB"), p("VTS_01_2.VOB"), p("VTS_01_3.VOB")}, "|"), InputPath(p("VTS_01_1.VOB")))
	assert.Equal(t, p("VTS_04_1.VOB"), InputPath(p("VTS_04_1.VOB")), "single parts are read directly")
	assert.Equal(t, "/videos/race.mkv", InputPath("/videos/race.mkv"))
	assert.Equal(t, "color=c=red:s=1x1", InputPath("color=c=red:s=1x1"), "lavfi sources pass through")

	args := Args{}.Input(p("VTS_01_1.VOB"))
	assert.Equal(t, "-i", args[0])
	assert.True(t, strings.HasPrefix(args[1], "concat:"), "every ffmpeg input is joined")
}

// TestDVDConcatPlayback writes a real MPEG-2 programme stream, splits it at
// byte boundaries like a DVD, and checks that ffprobe sees the full length and
// that ffmpeg can seek into the last part.
func TestDVDConcatPlayback(t *testing.T) {
	ffmpegPath, err1 := exec.LookPath("ffmpeg")
	ffprobePath, err2 := exec.LookPath("ffprobe")
	if err1 != nil || err2 != nil {
		t.Skip("ffmpeg and ffprobe are needed")
	}

	dir := filepath.Join(t.TempDir(), "VIDEO_TS")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	full := filepath.Join(t.TempDir(), "full.vob")

	out, err := exec.Command(ffmpegPath, "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=duration=30:size=352x288:rate=25",
		"-f", "lavfi", "-i", "sine=duration=30",
		"-shortest", "-target", "pal-dvd", "-b:v", "3000k", "-y", full).CombinedOutput()
	require.NoError(t, err, string(out))

	data, err := os.ReadFile(full)
	require.NoError(t, err)
	third := len(data)/3 + 1
	for i := 0; i < 3; i++ {
		end := min((i+1)*third, len(data))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "VTS_01_"+strconv.Itoa(i+1)+".VOB"), data[i*third:end], 0o644))
	}
	first := filepath.Join(dir, "VTS_01_1.VOB")

	duration := func(input string) float64 {
		out, err := exec.Command(ffprobePath, "-v", "error", "-show_entries", "format=duration", "-of", "default=nw=1:nk=1", input).Output()
		require.NoError(t, err)
		d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		require.NoError(t, err)
		return d
	}

	partOnly := duration(first)
	whole := duration(InputPath(first))
	assert.Less(t, partOnly, 15.0, "a single part is only a third")
	assert.InDelta(t, 30.0, whole, 1.0, "the joined title has the full length")

	frame := filepath.Join(t.TempDir(), "frame.jpg")
	args := Args{}.Seek(27).Input(first).VideoFrames(1).Overwrite().Output(frame)
	out, err = exec.Command(ffmpegPath, append([]string{"-loglevel", "error"}, args...)...).CombinedOutput()
	require.NoError(t, err, string(out))
	info, err := os.Stat(frame)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0), "a frame from the last part")
}
