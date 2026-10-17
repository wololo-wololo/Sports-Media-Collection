package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stashapp/stash/pkg/models"
)

type want struct {
	year, round, part int
	series            string
	session           models.SessionType
	sessionName       string
	feed              models.FeedType
	subject           string
	broadcaster       string
	bLang             string
	ambiguous         bool
	language          string
	source            models.RecordingSource
	dvd               bool
	words             []string
}

func TestParse(t *testing.T) {
	tests := []struct {
		path string
		want want
	}{
		{`D:\Motorsport\F1\2024\Formula1.2024.Round21.Sao.Paulo.Race.SkyF1.1080p50.mkv`,
			want{year: 2024, round: 21, series: "Formula 1", session: models.SessionTypeRace, broadcaster: "Sky Sports F1", bLang: "en", words: []string{"sao", "paulo"}}},
		{`D:\Motorsport\F1\2024\F1.2024.R22.Las.Vegas.Qualifying.SkyF1.1080p.mkv`,
			want{year: 2024, round: 22, series: "Formula 1", session: models.SessionTypeQualifying, broadcaster: "Sky Sports F1", bLang: "en", words: []string{"las", "vegas"}}},
		{`D:\Motorsport\F1\2024\Onboard_PIA_Race_LasVegas2024.mp4`,
			want{year: 2024, series: "Formula 1", session: models.SessionTypeRace, feed: models.FeedTypeOnboard, subject: "PIA", words: []string{"las", "vegas"}}},
		{`D:\Motorsport\DVD\F1 1999 Nürburgring RTL\VIDEO_TS\VTS_01_1.VOB`,
			want{year: 1999, series: "Formula 1", broadcaster: "RTL", bLang: "de", source: models.RecordingSourceDVD, dvd: true, words: []string{"nurburgring"}}},
		{`E:\Captures\gp_brasil_sprint.ts`,
			want{session: models.SessionTypeSprint, source: models.RecordingSourceTV, words: []string{"brasil"}}},
		{`D:\Motorsport\WEC\2023 Le Mans part 4.mkv`,
			want{year: 2023, part: 4, series: "WEC", words: []string{"mans"}}},
		{`F1 2024 Interlagos Qualifying Viaplay NL.mkv`,
			want{year: 2024, series: "Formula 1", session: models.SessionTypeQualifying, broadcaster: "Viaplay", bLang: "nl", language: "nl", words: []string{"interlagos"}}},
		{`GP Brasilien 2024 - 1. Training - Sky DE.mkv`,
			want{year: 2024, session: models.SessionTypePractice1, broadcaster: "Sky Deutschland", bLang: "de", words: []string{"brasilien"}}},
		{`Formel 1 - Großer Preis von Japan 1998 - Rennen - RTL.avi`,
			want{year: 1998, series: "Formula 1", session: models.SessionTypeRace, broadcaster: "RTL", bLang: "de", words: []string{"japan"}}},
		{`2021 Abu Dhabi GP Pre-Race Show Sky Sports F1.mkv`,
			want{year: 2021, feed: models.FeedTypeStudio, broadcaster: "Sky Sports F1", bLang: "en", words: []string{"abu", "dhabi", "show"}}},
		{`F1 2023 Azerbaijan Sprint Shootout.mkv`,
			want{year: 2023, series: "Formula 1", session: models.SessionTypeSprintQualifying, sessionName: "Sprint Shootout", words: []string{"azerbaijan"}}},
		{`Monaco 2024 FP2 F1TV Data Channel.mkv`,
			want{year: 2024, session: models.SessionTypePractice2, feed: models.FeedTypeDataChannel, broadcaster: "F1 TV", bLang: "en", words: []string{"monaco"}}},
		{`Gran Premio de España 2024 Carrera Movistar.mp4`,
			want{year: 2024, session: models.SessionTypeRace, broadcaster: "Movistar", bLang: "es", words: []string{"espana"}}},
		{`German GP 1998 RTL.mkv`,
			want{year: 1998, broadcaster: "RTL", bLang: "de", words: []string{"german"}}},
		{`2024 Brazil Q3.mkv`,
			want{year: 2024, session: models.SessionTypeQualifying, words: []string{"brazil"}}},
		{`Season Review 2008 BBC.mkv`,
			want{year: 2008, feed: models.FeedTypeDocumentary, broadcaster: "BBC", bLang: "en"}},
		{`Bahrain 2024 Race 1080p.mkv`,
			want{year: 2024, session: models.SessionTypeRace, words: []string{"bahrain"}}},
		{`F1 2024 Bahrain P1 Sky.mkv`,
			want{year: 2024, series: "Formula 1", session: models.SessionTypePractice1, broadcaster: "Sky Sports F1", bLang: "en", ambiguous: true, words: []string{"bahrain"}}},
		{`MotoGP 2015 Valencia Race ENG.mp4`,
			want{year: 2015, series: "MotoGP", session: models.SessionTypeRace, language: "en", words: []string{"valencia"}}},
		{`/media/f1/1998/16 - Japan/Rennen.mkv`,
			want{year: 1998, series: "Formula 1", session: models.SessionTypeRace, words: []string{"japan"}}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			p := Parse(tt.path, nil)
			w := tt.want
			assert.Equal(t, w.year, p.Year, "year")
			assert.Equal(t, w.round, p.Round, "round")
			assert.Equal(t, w.part, p.Part, "part")
			assert.Equal(t, w.series, p.Series, "series")
			assert.Equal(t, w.session, p.Session, "session")
			assert.Equal(t, w.sessionName, p.SessionName, "session name")
			assert.Equal(t, w.feed, p.Feed, "feed")
			assert.Equal(t, w.subject, p.FeedSubject, "feed subject")
			assert.Equal(t, w.broadcaster, p.Broadcaster, "broadcaster")
			assert.Equal(t, w.bLang, p.BroadcasterLanguage, "broadcaster language")
			assert.Equal(t, w.ambiguous, p.BroadcasterAmbiguous, "ambiguous broadcaster")
			assert.Equal(t, w.language, p.Language, "language")
			assert.Equal(t, w.source, p.Source, "source")
			assert.Equal(t, w.dvd, p.IsDVD, "dvd")
			assert.Equal(t, w.words, p.EventWords, "event words")
		})
	}
}

func TestSplitWords(t *testing.T) {
	assert.Equal(t, []string{"Sky", "F", "1"}, splitWords("SkyF1"))
	assert.Equal(t, []string{"Las", "Vegas", "2024"}, splitWords("LasVegas2024"))
	assert.Equal(t, []string{"TV", "Rip"}, splitWords("TVRip"))
	assert.Equal(t, []string{"1080", "p", "50"}, splitWords("1080p50"))
	assert.Equal(t, []string{"Grosser", "Preis"}, splitWords("Großer Preis"))
	assert.Equal(t, []string{"Canal", "plus", "FR"}, splitWords("Canal+ FR"))
}

func TestRules(t *testing.T) {
	rules, err := CompileRules([]string{
		"{year}-{round} {event} [{session}] {broadcaster}",
		"{series} {year} {event} {feed} {driver} {lang}",
	})
	require.NoError(t, err)

	p := Parse(`/f1/2024-21 Sao Paulo [Race] Sky Sports F1.mkv`, rules)
	assert.Equal(t, 2024, p.Year)
	assert.Equal(t, 21, p.Round)
	assert.Equal(t, models.SessionTypeRace, p.Session)
	assert.Equal(t, "Sky Sports F1", p.Broadcaster)
	assert.Equal(t, []string{"sao", "paulo"}, p.EventWords)
	assert.Equal(t, OriginRule, p.Origins["round"])

	p = Parse(`/f1/2024-03 Melbourne [Rennen] Canal 9 Australia.mkv`, rules)
	assert.Equal(t, "Canal+", p.Broadcaster, "known broadcaster in the captured text")

	p = Parse(`/f1/2024-03 Melbourne [Rennen] Channel Nine.mkv`, rules)
	assert.Equal(t, "Channel Nine", p.Broadcaster, "unknown broadcasters are kept as written")
	assert.Equal(t, models.SessionTypeRace, p.Session)

	_, err = CompileRules([]string{"{year} {nonsense}", "no placeholders", "{year} {event}"})
	assert.Error(t, err)
}
