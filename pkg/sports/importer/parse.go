// Package importer recognises sports recordings from their file and folder
// names and matches them to the series calendars.
package importer

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/stashapp/stash/pkg/models"
)

// Origin says where a parsed value came from.
type Origin string

const (
	OriginFilename Origin = "filename"
	OriginFolder   Origin = "folder"
	OriginRule     Origin = "rule"
	// OriginGuess marks values inferred from indirect hints such as the
	// file extension.
	OriginGuess Origin = "guess"
)

// Parsed holds what could be read from a recording's path. Zero values mean
// "not found".
type Parsed struct {
	Year  int
	Round int
	Part  int

	// Series is a canonical series name such as "Formula 1".
	Series string

	Session     models.SessionType
	SessionName string

	Feed        models.FeedType
	FeedSubject string

	Broadcaster          string
	BroadcasterLanguage  string
	BroadcasterAmbiguous bool
	Language             string
	Source               models.RecordingSource
	IsDVD                bool
	EventWords           []string
	Origins              map[string]Origin
}

func (p *Parsed) setOrigin(field string, o Origin) {
	if p.Origins == nil {
		p.Origins = make(map[string]Origin)
	}
	p.Origins[field] = o
}

// ---------------------------------------------------------------------------
// tokenising

type token struct {
	text  string // lower-case, accent-free
	upper bool   // written in capitals
	used  bool
}

var letterMap = strings.NewReplacer("ß", "ss", "æ", "ae", "œ", "oe", "ø", "o", "ł", "l", "đ", "d", "ı", "i", "+", " plus ")

// splitWords splits a name into words at separators, case changes
// ("SkyF1" -> "Sky", "F", "1") and letter/digit boundaries.
func splitWords(s string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, string(cur))
			cur = cur[:0]
		}
	}

	runes := []rune(letterMap.Replace(s))
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			continue
		}
		if len(cur) > 0 {
			prev := cur[len(cur)-1]
			switch {
			case unicode.IsDigit(prev) != unicode.IsDigit(r):
				flush()
			case unicode.IsLower(prev) && unicode.IsUpper(r):
				flush()
			case unicode.IsUpper(prev) && unicode.IsUpper(r) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) && len(cur) > 1:
				// "WEBDl" stays, "TVRip" -> "TV", "Rip"
				flush()
			}
		}
		cur = append(cur, r)
	}
	flush()
	return words
}

func fold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func tokenise(s string) []token {
	words := splitWords(s)
	ret := make([]token, 0, len(words))
	for _, w := range words {
		upper := strings.ToUpper(w) == w && strings.ToLower(w) != w
		ret = append(ret, token{text: fold(w), upper: upper})
	}
	return ret
}

func phraseTokens(p string) []string {
	var ret []string
	for _, t := range tokenise(p) {
		ret = append(ret, t.text)
	}
	return ret
}

// find returns the start of the first unused occurrence of phrase.
func find(toks []token, phrase []string) int {
	if len(phrase) == 0 {
		return -1
	}
outer:
	for i := 0; i+len(phrase) <= len(toks); i++ {
		for j, p := range phrase {
			if toks[i+j].used || toks[i+j].text != p {
				continue outer
			}
		}
		return i
	}
	return -1
}

func use(toks []token, start, n int) {
	for i := start; i < start+n; i++ {
		toks[i].used = true
	}
}

// matchPhrases finds the first phrase of the list in toks, marks it used and
// returns its index in phrases.
func matchPhrases(toks []token, phrases []string) (int, int, bool) {
	best, bestStart, bestLen := -1, -1, 0
	for i, p := range phrases {
		pt := phraseTokens(p)
		if s := find(toks, pt); s >= 0 && len(pt) > bestLen {
			best, bestStart, bestLen = i, s, len(pt)
		}
	}
	if best < 0 {
		return -1, -1, false
	}
	use(toks, bestStart, bestLen)
	return best, bestStart, true
}

func number(t token, lo, hi int) (int, bool) {
	n, err := strconv.Atoi(t.text)
	if err != nil || n < lo || n > hi {
		return 0, false
	}
	return n, true
}

// ---------------------------------------------------------------------------
// field extraction

func (p *Parsed) extract(toks []token, o Origin) {
	if p.Broadcaster == "" {
		for _, b := range broadcasterVocab {
			if _, _, ok := matchPhrases(toks, b.phrases); ok {
				p.Broadcaster = b.info.Name
				p.BroadcasterLanguage = b.info.Language
				p.BroadcasterAmbiguous = b.info.Ambiguous
				p.setOrigin("broadcaster", o)
				break
			}
		}
	}

	if p.Series == "" {
		for _, s := range seriesVocab {
			if _, _, ok := matchPhrases(toks, s.phrases); ok {
				p.Series = s.name
				p.setOrigin("series", o)
				break
			}
		}
	}

	p.extractNumbered(toks, o)

	// feeds before sessions, so that "Pre-Race Show" is a studio programme
	// rather than the race
	onboardAt := -1
	if p.Feed == "" {
		for _, f := range feedVocab {
			if _, start, ok := matchPhrases(toks, f.phrases); ok {
				p.Feed = f.ft
				p.setOrigin("feed", o)
				if f.ft == models.FeedTypeOnboard {
					onboardAt = start
				}
				break
			}
		}
	}

	if p.Session == "" {
		for _, s := range sessionVocab {
			if _, _, ok := matchPhrases(toks, s.phrases); ok {
				p.Session = s.st
				p.SessionName = s.name
				p.setOrigin("session", o)
				break
			}
		}
	}
	if p.Session == "" {
		// a lone capital Q, e.g. "Monaco 2024 Q Sky"
		for i := range toks {
			if !toks[i].used && toks[i].upper && toks[i].text == "q" {
				toks[i].used = true
				p.Session = models.SessionTypeQualifying
				p.setOrigin("session", o)
				break
			}
		}
	}

	if p.Source == "" {
		for _, s := range sourceVocab {
			if _, _, ok := matchPhrases(toks, s.phrases); ok {
				p.Source = s.src
				p.setOrigin("source", o)
				break
			}
		}
	}

	if p.Language == "" {
		for i := range toks {
			t := &toks[i]
			if t.used {
				continue
			}
			lang, ok := languageWords[t.text]
			if !ok && t.upper && len(t.text) == 2 {
				lang, ok = languageCodes[t.text]
			}
			if ok {
				t.used = true
				p.Language = lang
				p.setOrigin("language", o)
				break
			}
		}
	}

	if p.Year == 0 {
		for i := range toks {
			if toks[i].used || len(toks[i].text) != 4 {
				continue
			}
			if y, ok := number(toks[i], 1950, 2099); ok {
				toks[i].used = true
				p.Year = y
				p.setOrigin("year", o)
				break
			}
		}
	}

	// the driver of an onboard, once every other word has been claimed
	if onboardAt >= 0 && p.FeedSubject == "" {
		p.FeedSubject = onboardSubject(toks, onboardAt)
	}
}

// extractNumbered reads round, part and numbered sessions ("FP2", "Q3",
// "1. Training", "Round 5").
func (p *Parsed) extractNumbered(toks []token, o Origin) {
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.used {
			continue
		}

		next := func(lo, hi int) (int, bool) {
			if i+1 < len(toks) && !toks[i+1].used {
				return number(toks[i+1], lo, hi)
			}
			return 0, false
		}

		switch t.text {
		case "r", "rd", "round", "lauf":
			if n, ok := next(1, 30); ok && p.Round == 0 {
				use(toks, i, 2)
				p.Round = n
				p.setOrigin("round", o)
			}
			continue
		case "part", "pt", "teil", "cd", "disc", "disk":
			if n, ok := next(1, 50); ok && p.Part == 0 {
				use(toks, i, 2)
				p.Part = n
				p.setOrigin("part", o)
			}
			continue
		case "q":
			// Q1, Q2 and Q3 are parts of qualifying
			if _, ok := next(1, 3); ok && p.Session == "" {
				use(toks, i, 2)
				p.Session = models.SessionTypeQualifying
				p.setOrigin("session", o)
			}
			continue
		case "sq":
			if _, ok := next(1, 3); ok && p.Session == "" {
				use(toks, i, 2)
				p.Session = models.SessionTypeSprintQualifying
				p.setOrigin("session", o)
			}
			continue
		case "p":
			// "P1".."P3" after a word, but not "1080p"
			if i > 0 && toks[i-1].text != "" && isDigits(toks[i-1].text) {
				continue
			}
			if n, ok := next(1, 3); ok && p.Session == "" {
				use(toks, i, 2)
				p.setPractice(n, o)
			}
			continue
		}
	}

	if p.Session != "" {
		return
	}

	// "practice 2", "FP2", "EL3" or "1. Training", "2nd practice"
	for _, phrase := range practicePhrases {
		pt := phraseTokens(phrase)
		s := find(toks, pt)
		if s < 0 {
			continue
		}
		if a := s + len(pt); a < len(toks) && !toks[a].used {
			if n, ok := number(toks[a], 1, 3); ok {
				use(toks, s, len(pt)+1)
				p.setPractice(n, o)
				return
			}
		}
		if b := s - 1; b >= 0 && !toks[b].used {
			if n, ok := number(toks[b], 1, 3); ok {
				use(toks, b, len(pt)+1)
				p.setPractice(n, o)
				return
			}
			if b-1 >= 0 && !toks[b-1].used && (toks[b].text == "st" || toks[b].text == "nd" || toks[b].text == "rd") {
				if n, ok := number(toks[b-1], 1, 3); ok {
					use(toks, b-1, len(pt)+2)
					p.setPractice(n, o)
					return
				}
			}
		}
		if len(pt) > 1 || phrase == "practice" || phrase == "training" {
			use(toks, s, len(pt))
			p.Session = models.SessionTypePractice
			p.setOrigin("session", o)
			return
		}
	}
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func (p *Parsed) setPractice(n int, o Origin) {
	switch n {
	case 1:
		p.Session = models.SessionTypePractice1
	case 2:
		p.Session = models.SessionTypePractice2
	case 3:
		p.Session = models.SessionTypePractice3
	}
	p.setOrigin("session", o)
}

// onboardSubject returns the driver next to an onboard phrase: a three
// letter code ("VER") or a name.
func onboardSubject(toks []token, start int) string {
	candidate := func(i int) (string, bool) {
		if i < 0 || i >= len(toks) || toks[i].used || isDigits(toks[i].text) || len(toks[i].text) < 3 || stopWords[toks[i].text] {
			return "", false
		}
		if _, ok := languageWords[toks[i].text]; ok {
			return "", false
		}
		toks[i].used = true
		if len(toks[i].text) == 3 {
			return strings.ToUpper(toks[i].text), true
		}
		return strings.ToUpper(toks[i].text[:1]) + toks[i].text[1:], true
	}

	end := start
	for end < len(toks) && toks[end].used {
		end++
	}
	if s, ok := candidate(end); ok {
		return s
	}
	if s, ok := candidate(start - 1); ok {
		return s
	}
	return ""
}

// eventWords returns the unused words that may name the event.
func eventWords(toks []token) []string {
	var ret []string
	for _, t := range toks {
		if t.used || isDigits(t.text) || len(t.text) < 2 || stopWords[t.text] {
			continue
		}
		ret = append(ret, t.text)
	}
	return ret
}

// ---------------------------------------------------------------------------
// paths

var vtsName = regexp.MustCompile(`(?i)^(vts_\d+_\d+|video_ts)$`)

// maxFolders is how many parent folders are read for missing values.
const maxFolders = 3

func splitPath(path string) []string {
	path = strings.ReplaceAll(path, `\`, "/")
	var ret []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			ret = append(ret, s)
		}
	}
	return ret
}

// Parse reads what it can from a recording's path. Rules are tried on the
// file name first; their values win over the built-in recognition.
func Parse(path string, rules []*Rule) Parsed {
	var p Parsed

	segs := splitPath(path)
	if len(segs) == 0 {
		return p
	}

	base := segs[len(segs)-1]
	ext := strings.ToLower(filepath.Ext(base))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	folders := segs[:len(segs)-1]

	// a DVD is named by the folder above VIDEO_TS
	if ext == ".vob" || ext == ".ifo" || vtsName.MatchString(base) || (len(folders) > 0 && strings.EqualFold(folders[len(folders)-1], "video_ts")) {
		p.IsDVD = true
		if len(folders) > 0 && strings.EqualFold(folders[len(folders)-1], "video_ts") {
			folders = folders[:len(folders)-1]
		}
		if len(folders) > 0 {
			base = folders[len(folders)-1]
			folders = folders[:len(folders)-1]
		}
	}
	if ext == ".iso" {
		p.IsDVD = true
	}

	for _, r := range rules {
		if r.apply(base, &p) {
			break
		}
	}

	toks := tokenise(base)
	p.extract(toks, OriginFilename)
	if words := eventWords(toks); len(p.EventWords) == 0 {
		p.EventWords = words
	}

	for i := len(folders) - 1; i >= 0 && i >= len(folders)-maxFolders; i-- {
		ft := tokenise(folders[i])
		p.extract(ft, OriginFolder)
		if len(p.EventWords) == 0 {
			p.EventWords = eventWords(ft)
		}
	}

	if p.IsDVD && p.Source == "" {
		p.Source = models.RecordingSourceDVD
		p.setOrigin("source", OriginFilename)
	}
	if p.Source == "" && ext == ".ts" {
		p.Source = models.RecordingSourceTV
		p.setOrigin("source", OriginGuess)
	}

	return p
}
