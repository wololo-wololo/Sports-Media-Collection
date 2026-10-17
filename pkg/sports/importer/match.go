package importer

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

// Confidence rates a suggestion.
type Confidence string

const (
	// ConfidenceHigh: the event and session were read from the name and
	// confirmed by the calendar.
	ConfidenceHigh Confidence = "HIGH"
	// ConfidenceMedium: the event is clear but some values were guessed.
	ConfidenceMedium Confidence = "MEDIUM"
	// ConfidenceLow: several events fit; one has to be chosen.
	ConfidenceLow Confidence = "LOW"
	// ConfidenceNone: no event could be found.
	ConfidenceNone Confidence = "NONE"
)

var confidenceRank = map[Confidence]int{ConfidenceNone: 0, ConfidenceLow: 1, ConfidenceMedium: 2, ConfidenceHigh: 3}

// AtLeast reports whether c is at least min.
func (c Confidence) AtLeast(min Confidence) bool {
	return confidenceRank[c] >= confidenceRank[min]
}

// Broadcaster is an existing studio that may match a parsed broadcaster.
type Broadcaster struct {
	ID      int
	Name    string
	Aliases []string
}

// Field describes one suggested value for display.
type Field struct {
	Name    string
	Value   string
	Guessed bool
	Origin  string
}

// Suggestion is what the matcher proposes for one recording.
type Suggestion struct {
	Confidence Confidence
	Series     *models.Series
	Event      *models.Event
	Session    *models.EventSession
	// Alternatives are other events that fit, best first.
	Alternatives []*models.Event

	// StudioID is set when the broadcaster exists; otherwise
	// BroadcasterName is the studio to create.
	StudioID        *int
	BroadcasterName string

	Language    string
	FeedType    *models.FeedType
	FeedSubject string
	Source      *models.RecordingSource
	Part        int

	Fields []Field
	Notes  []string
}

type calendarEvent struct {
	event    *models.Event
	series   *models.Series
	sessions []*models.EventSession
	// text is the event's searchable words, space separated with a leading
	// and trailing space for whole-word lookups.
	text string
	date time.Time
}

// Matcher matches recordings against a snapshot of the calendar.
type Matcher struct {
	series       []*models.Series
	events       []*calendarEvent
	eventsByID   map[int]*calendarEvent
	broadcasters map[string]Broadcaster
	rules        []*Rule
}

func compact(s string) string {
	var b strings.Builder
	for _, t := range tokenise(s) {
		b.WriteString(t.text)
	}
	return b.String()
}

func searchText(parts ...string) string {
	var words []string
	for _, p := range parts {
		for _, t := range tokenise(p) {
			words = append(words, t.text)
		}
	}
	return " " + strings.Join(words, " ") + " "
}

// NewMatcher indexes the calendar. sessions may hold sessions of any event;
// those of unknown events are ignored.
func NewMatcher(series []*models.Series, events []*models.Event, sessions []*models.EventSession, broadcasters []Broadcaster, rules []*Rule) *Matcher {
	m := &Matcher{series: series, broadcasters: make(map[string]Broadcaster), rules: rules}

	seriesByID := make(map[int]*models.Series)
	for _, s := range series {
		seriesByID[s.ID] = s
	}

	byID := make(map[int]*calendarEvent)
	m.eventsByID = byID
	for _, e := range events {
		s := seriesByID[e.SeriesID]
		if s == nil {
			continue
		}
		ce := &calendarEvent{event: e, series: s, text: searchText(e.Name, e.Location, e.Country, e.Circuit)}
		if e.EndDate != nil {
			ce.date = e.EndDate.Time
		} else if e.StartDate != nil {
			ce.date = e.StartDate.Time
		}
		m.events = append(m.events, ce)
		byID[e.ID] = ce
	}
	for _, s := range sessions {
		if ce := byID[s.EventID]; ce != nil {
			ce.sessions = append(ce.sessions, s)
		}
	}

	for _, b := range broadcasters {
		m.broadcasters[compact(b.Name)] = b
		for _, a := range b.Aliases {
			if k := compact(a); k != "" {
				if _, taken := m.broadcasters[k]; !taken {
					m.broadcasters[k] = b
				}
			}
		}
	}

	return m
}

func (m *Matcher) findSeries(name string) *models.Series {
	key := compact(name)
	for _, s := range m.series {
		if compact(s.Name) == key || (s.ShortName != "" && compact(s.ShortName) == key) {
			return s
		}
	}
	// "F1" names Formula 1, whatever the user called the series
	for _, sv := range seriesVocab {
		if sv.name != name {
			continue
		}
		for _, s := range m.series {
			for _, ph := range sv.phrases {
				if compact(s.Name) == compact(ph) || (s.ShortName != "" && compact(s.ShortName) == compact(ph)) {
					return s
				}
			}
		}
	}
	return nil
}

// wordScore counts the parsed words that name the event, directly or
// through an alias. Two-word names such as "Sao Paulo" count once per word.
func wordScore(words []string, text string) int {
	covered := make([]bool, len(words))
	has := func(phrase string) bool {
		return phrase != "" && strings.Contains(text, " "+phrase+" ")
	}
	try := func(phrase string, idx ...int) {
		ok := has(phrase)
		if !ok {
			for _, target := range eventAliases[phrase] {
				if has(searchText(target)[1 : len(searchText(target))-1]) {
					ok = true
					break
				}
			}
		}
		if ok {
			for _, i := range idx {
				covered[i] = true
			}
		}
	}

	for i := 0; i+1 < len(words); i++ {
		try(words[i]+" "+words[i+1], i, i+1)
	}
	for i, w := range words {
		if !covered[i] && len(w) >= 3 {
			try(w, i)
		}
	}

	n := 0
	for _, c := range covered {
		if c {
			n++
		}
	}
	return n
}

func (ce *calendarEvent) session(t models.SessionType) *models.EventSession {
	for _, s := range ce.sessions {
		if s.SessionType == t {
			return s
		}
	}
	// a calendar without numbered practice still has the session
	if t == models.SessionTypePractice {
		for _, s := range ce.sessions {
			if s.SessionType == models.SessionTypePractice1 {
				return s
			}
		}
	}
	return nil
}

func daysApart(a, b time.Time) float64 {
	d := a.Sub(b).Hours() / 24
	if d < 0 {
		return -d
	}
	return d
}

func sessionLabel(t models.SessionType) string {
	switch t {
	case models.SessionTypePractice1:
		return "Practice 1"
	case models.SessionTypePractice2:
		return "Practice 2"
	case models.SessionTypePractice3:
		return "Practice 3"
	case models.SessionTypeSprintQualifying:
		return "Sprint qualifying"
	case models.SessionTypeWarmUp:
		return "Warm-up"
	}
	s := strings.ToLower(strings.ReplaceAll(string(t), "_", " "))
	return strings.ToUpper(s[:1]) + s[1:]
}

func eventLabel(e *models.Event) string {
	if e.Round != nil {
		return fmt.Sprintf("%d %s · R%d", e.Season, e.Name, *e.Round)
	}
	return fmt.Sprintf("%d %s", e.Season, e.Name)
}

// Match suggests calendar links and recording fields for a path. fileDate is
// the file's modification time, used to rank events when no year is known.
func (m *Matcher) Match(path string, fileDate time.Time) Suggestion {
	return m.match(path, fileDate, nil)
}

// MatchEvent is Match with the event chosen by the user, e.g. one of the
// alternatives of a low-confidence suggestion.
func (m *Matcher) MatchEvent(path string, fileDate time.Time, eventID int) (Suggestion, error) {
	ce := m.eventsByID[eventID]
	if ce == nil {
		return Suggestion{}, fmt.Errorf("event with id %d not found", eventID)
	}
	return m.match(path, fileDate, ce), nil
}

func (m *Matcher) match(path string, fileDate time.Time, forced *calendarEvent) Suggestion {
	p := Parse(path, m.rules)
	var s Suggestion
	s.Part = p.Part

	guessed := map[string]bool{}
	origin := func(f string) string {
		if o, ok := p.Origins[f]; ok {
			return string(o)
		}
		return string(OriginGuess)
	}

	// --- series
	var candidates []*calendarEvent
	if p.Series != "" {
		s.Series = m.findSeries(p.Series)
		if s.Series == nil {
			s.Notes = append(s.Notes, fmt.Sprintf("%s is not in the library yet", p.Series))
		}
	}
	for _, ce := range m.events {
		if s.Series != nil && ce.series.ID != s.Series.ID {
			continue
		}
		if p.Series != "" && s.Series == nil {
			continue
		}
		if p.Year != 0 && ce.event.Season != p.Year {
			continue
		}
		if p.Round != 0 && (ce.event.Round == nil || *ce.event.Round != p.Round) {
			continue
		}
		candidates = append(candidates, ce)
	}

	// --- event
	type scored struct {
		ce    *calendarEvent
		score int
		// hasSession: the event held the session the name mentions, which
		// tells e.g. sprint weekends apart when the year is unknown
		hasSession bool
	}
	var ranked []scored
	for _, ce := range candidates {
		sc := wordScore(p.EventWords, ce.text)
		if sc == 0 && p.Round == 0 {
			continue
		}
		if sc == 0 && p.Year == 0 {
			continue
		}
		has := p.Session == "" || ce.session(p.Session) != nil
		ranked = append(ranked, scored{ce, sc, has})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].hasSession != ranked[j].hasSession {
			return ranked[i].hasSession
		}
		if !fileDate.IsZero() {
			return daysApart(ranked[i].ce.date, fileDate) < daysApart(ranked[j].ce.date, fileDate)
		}
		return ranked[i].ce.event.Season > ranked[j].ce.event.Season
	})

	var best []*calendarEvent
	for _, r := range ranked {
		if r.score == ranked[0].score && r.hasSession == ranked[0].hasSession {
			best = append(best, r.ce)
		}
	}

	var chosen *calendarEvent
	switch {
	case forced != nil:
		chosen = forced
	case len(best) == 1:
		chosen = best[0]
	case len(best) > 1:
		chosen = best[0]
		s.Confidence = ConfidenceLow
		for _, ce := range best {
			s.Alternatives = append(s.Alternatives, ce.event)
			if len(s.Alternatives) == 5 {
				break
			}
		}
		if p.Year == 0 {
			s.Notes = append(s.Notes, "No year in the name; pick the right weekend")
		} else {
			s.Notes = append(s.Notes, "Several events fit; pick the right one")
		}
	}

	if chosen == nil {
		s.Confidence = ConfidenceNone
		switch {
		case len(p.EventWords) == 0 && p.Round == 0:
			s.Notes = append(s.Notes, "The name does not say which event this is")
		case len(m.events) == 0:
			s.Notes = append(s.Notes, "The calendar is empty; sync it first")
		default:
			s.Notes = append(s.Notes, "No event in the calendar matches the name")
		}
	} else if forced != nil {
		// the user's choice needs no guessing
		s.Event = chosen.event
		s.Series = chosen.series
	} else {
		s.Event = chosen.event
		s.Series = chosen.series
		if p.Series == "" {
			guessed["series"] = true
		}
		if p.Year == 0 {
			guessed["event"] = true
		}
		if p.Round != 0 && len(p.EventWords) > 0 && wordScore(p.EventWords, chosen.text) == 0 {
			guessed["event"] = true
			s.Notes = append(s.Notes, fmt.Sprintf("Round %d is %s, which the name does not mention", p.Round, chosen.event.Name))
		}
	}

	// --- session
	if chosen != nil {
		st := p.Session
		studioLike := p.Feed == models.FeedTypeStudio || p.Feed == models.FeedTypeDocumentary
		if st == "" && !studioLike {
			st = models.SessionTypeRace
			guessed["session"] = true
		}
		if st != "" {
			s.Session = chosen.session(st)
			if s.Session == nil && !guessed["session"] {
				guessed["session"] = true
				s.Notes = append(s.Notes, fmt.Sprintf("The calendar has no %s session for this event", strings.ToLower(sessionLabel(st))))
			}
		}
	}

	// --- broadcaster
	if p.Broadcaster != "" {
		name := p.Broadcaster
		if p.BroadcasterAmbiguous {
			switch p.Language {
			case "de":
				name = "Sky Deutschland"
			case "it":
				name = "Sky Sport Italia"
			default:
				guessed["broadcaster"] = true
			}
		}
		if b, ok := m.broadcasters[compact(name)]; ok {
			id := b.ID
			s.StudioID = &id
			name = b.Name
		}
		s.BroadcasterName = name
	}

	// --- language, feed, source
	s.Language = p.Language
	if s.Language == "" && p.BroadcasterLanguage != "" && p.Feed != models.FeedTypeOnboard {
		s.Language = p.BroadcasterLanguage
		if p.BroadcasterAmbiguous && p.Language == "" {
			s.Language = "en"
		}
		guessed["language"] = true
	}

	if p.Feed != "" {
		f := p.Feed
		s.FeedType = &f
		s.FeedSubject = p.FeedSubject
	} else if p.Broadcaster != "" {
		f := models.FeedTypeWorldFeed
		s.FeedType = &f
		guessed["feed"] = true
	}

	if p.Source != "" {
		src := p.Source
		s.Source = &src
		if p.Origins["source"] == OriginGuess {
			guessed["source"] = true
		}
	}

	// --- confidence
	if s.Confidence == "" {
		s.Confidence = ConfidenceHigh
		for _, f := range []string{"series", "event", "session", "broadcaster"} {
			if guessed[f] {
				s.Confidence = ConfidenceMedium
			}
		}
		if s.Session == nil && p.Feed != models.FeedTypeStudio && p.Feed != models.FeedTypeDocumentary {
			s.Confidence = ConfidenceMedium
		}
	}

	// --- display fields
	add := func(name, value, originField string) {
		if value == "" {
			return
		}
		o := origin(originField)
		if guessed[name] {
			o = string(OriginGuess)
		}
		s.Fields = append(s.Fields, Field{Name: name, Value: value, Guessed: guessed[name], Origin: o})
	}
	if s.Event != nil {
		add("event", eventLabel(s.Event), "year")
	}
	if s.Session != nil {
		label := sessionLabel(s.Session.SessionType)
		if s.Session.Name != "" {
			label = s.Session.Name
		}
		add("session", label, "session")
	}
	add("broadcaster", s.BroadcasterName, "broadcaster")
	add("language", s.Language, "language")
	if s.FeedType != nil {
		v := sessionLabel(models.SessionType(*s.FeedType))
		if s.FeedSubject != "" {
			v += " · " + s.FeedSubject
		}
		add("feed", v, "feed")
	}
	if s.Source != nil {
		add("source", string(*s.Source), "source")
	}
	if s.Part != 0 {
		add("part", strconv.Itoa(s.Part), "part")
	}

	return s
}
