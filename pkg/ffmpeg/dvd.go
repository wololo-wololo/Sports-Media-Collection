package ffmpeg

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// A ripped DVD keeps each title set in a VIDEO_TS folder as VTS_nn_0.VOB (the
// menu) followed by VTS_nn_1.VOB, VTS_nn_2.VOB, ... which together form one
// continuous MPEG program stream, split at about 1 GB. The first part stands
// for the whole title: it is what the library scans, and every ffmpeg and
// ffprobe call reads the parts joined with the concat protocol.

var dvdPart = regexp.MustCompile(`(?i)^VTS_(\d{2})_(\d)\.VOB$`)

// MinDVDTitleBytes is the size below which a title set is treated as an
// extra (trailer, logo, menu loop) rather than a recording; about two
// minutes of DVD video.
const MinDVDTitleBytes int64 = 100 << 20

// dvdTitleParts returns the parts of the title set whose first part is path,
// in order, with their total size. It returns nil if path is not the first
// part of a title set.
func dvdTitleParts(path string) ([]string, int64) {
	base := filepath.Base(path)
	m := dvdPart.FindStringSubmatch(base)
	if m == nil || m[2] != "1" {
		return nil, 0
	}

	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0
	}

	byPart := make(map[int]os.DirEntry)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		pm := dvdPart.FindStringSubmatch(e.Name())
		if pm == nil || pm[1] != m[1] {
			continue
		}
		n, _ := strconv.Atoi(pm[2])
		if n >= 1 {
			byPart[n] = e
		}
	}

	var parts []string
	var total int64
	// parts are consecutive; stop at the first gap
	for n := 1; ; n++ {
		e, ok := byPart[n]
		if !ok {
			break
		}
		info, err := e.Info()
		if err != nil {
			return nil, 0
		}
		parts = append(parts, filepath.Join(dir, e.Name()))
		total += info.Size()
	}

	if len(parts) == 0 {
		return nil, 0
	}
	return parts, total
}

// DVDTitleParts returns the VOB files making up the DVD title whose first
// part is path, or nil if path is not the first part of a DVD title set.
func DVDTitleParts(path string) []string {
	parts, _ := dvdTitleParts(path)
	return parts
}

// IsDVDVob reports whether path is a VOB file named like a DVD title set
// part or menu (VTS_nn_n.VOB or VIDEO_TS.VOB).
func IsDVDVob(path string) bool {
	base := filepath.Base(path)
	return dvdPart.MatchString(base) || strings.EqualFold(base, "VIDEO_TS.VOB")
}

// IsDVDTitle reports whether path is the first part of a DVD title set
// large enough to be a recording. Such files are scanned as videos; the other
// parts and the menus are not.
func IsDVDTitle(path string) bool {
	parts, total := dvdTitleParts(path)
	return len(parts) > 0 && total >= MinDVDTitleBytes
}

// InputPath returns the input to give ffmpeg or ffprobe for a video file:
// the joined parts for the first part of a multi-part DVD title, otherwise
// the path itself.
func InputPath(path string) string {
	parts := DVDTitleParts(path)
	if len(parts) < 2 {
		return path
	}
	return "concat:" + strings.Join(parts, "|")
}
