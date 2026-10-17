package models

import (
	"fmt"
	"io"
	"strconv"
	"time"
)

// Series is a competition such as Formula 1 or MotoGP. Events belong to a
// series and are grouped by season.
type Series struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
	// Sport the series belongs to, e.g. "Motorsport" or "Football".
	Sport string `json:"sport"`
	// Color is a display colour in hex notation, e.g. "#E8473C".
	Color string `json:"color"`
	// DataSource identifies where calendar data is synced from, e.g. "jolpica-f1".
	DataSource string    `json:"data_source"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func NewSeries() Series {
	currentTime := time.Now()
	return Series{
		Sport:     DefaultSport,
		CreatedAt: currentTime,
		UpdatedAt: currentTime,
	}
}

const DefaultSport = "Motorsport"

type SeriesPartial struct {
	Name       OptionalString
	ShortName  OptionalString
	Sport      OptionalString
	Color      OptionalString
	DataSource OptionalString
	CreatedAt  OptionalTime
	UpdatedAt  OptionalTime
}

func NewSeriesPartial() SeriesPartial {
	return SeriesPartial{
		UpdatedAt: NewOptionalTime(time.Now()),
	}
}

// Event is a single meeting within a series season, such as a Grand Prix.
type Event struct {
	ID       int    `json:"id"`
	SeriesID int    `json:"series_id"`
	Season   int    `json:"season"`
	Round    *int   `json:"round"`
	Name     string `json:"name"`
	Circuit  string `json:"circuit"`
	Location string `json:"location"`
	Country  string `json:"country"`
	// StartDate and EndDate are the first and last day of the event.
	StartDate *Date `json:"start_date"`
	EndDate   *Date `json:"end_date"`
	// ExternalID is the identifier used by the series data source, used to
	// match events when syncing calendars.
	ExternalID string    `json:"external_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func NewEvent() Event {
	currentTime := time.Now()
	return Event{
		CreatedAt: currentTime,
		UpdatedAt: currentTime,
	}
}

type EventPartial struct {
	SeriesID   OptionalInt
	Season     OptionalInt
	Round      OptionalInt
	Name       OptionalString
	Circuit    OptionalString
	Location   OptionalString
	Country    OptionalString
	StartDate  OptionalDate
	EndDate    OptionalDate
	ExternalID OptionalString
	CreatedAt  OptionalTime
	UpdatedAt  OptionalTime
}

func NewEventPartial() EventPartial {
	return EventPartial{
		UpdatedAt: NewOptionalTime(time.Now()),
	}
}

// EventSession is one session of an event, such as qualifying or the race.
type EventSession struct {
	ID          int         `json:"id"`
	EventID     int         `json:"event_id"`
	SessionType SessionType `json:"session_type"`
	// Name is an optional display name overriding the session type label,
	// e.g. "Sprint Shootout".
	Name      string     `json:"name"`
	StartTime *time.Time `json:"start_time"`
	// Winner of the session; the pole sitter for qualifying.
	Winner    string    `json:"winner"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewEventSession() EventSession {
	currentTime := time.Now()
	return EventSession{
		CreatedAt: currentTime,
		UpdatedAt: currentTime,
	}
}

type EventSessionPartial struct {
	EventID     OptionalInt
	SessionType OptionalString
	Name        OptionalString
	StartTime   OptionalTime
	Winner      OptionalString
	CreatedAt   OptionalTime
	UpdatedAt   OptionalTime
}

func NewEventSessionPartial() EventSessionPartial {
	return EventSessionPartial{
		UpdatedAt: NewOptionalTime(time.Now()),
	}
}

// enum is implemented by the string enums in this file.
type enum interface {
	~string
	IsValid() bool
}

func unmarshalEnum[T enum](e *T, v interface{}, name string) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = T(str)
	if !(*e).IsValid() {
		return fmt.Errorf("%s is not a valid %s", str, name)
	}
	return nil
}

func marshalEnum(w io.Writer, s string) {
	fmt.Fprint(w, strconv.Quote(s))
}

// SessionType is the kind of an event session.
type SessionType string

const (
	SessionTypePractice1        SessionType = "PRACTICE_1"
	SessionTypePractice2        SessionType = "PRACTICE_2"
	SessionTypePractice3        SessionType = "PRACTICE_3"
	SessionTypePractice         SessionType = "PRACTICE"
	SessionTypeQualifying       SessionType = "QUALIFYING"
	SessionTypeSprintQualifying SessionType = "SPRINT_QUALIFYING"
	SessionTypeSprint           SessionType = "SPRINT"
	SessionTypeWarmUp           SessionType = "WARM_UP"
	SessionTypeRace             SessionType = "RACE"
	SessionTypeMatch            SessionType = "MATCH"
	SessionTypeOther            SessionType = "OTHER"
)

var AllSessionType = []SessionType{
	SessionTypePractice1,
	SessionTypePractice2,
	SessionTypePractice3,
	SessionTypePractice,
	SessionTypeQualifying,
	SessionTypeSprintQualifying,
	SessionTypeSprint,
	SessionTypeWarmUp,
	SessionTypeRace,
	SessionTypeMatch,
	SessionTypeOther,
}

func (e SessionType) IsValid() bool {
	for _, v := range AllSessionType {
		if e == v {
			return true
		}
	}
	return false
}

func (e SessionType) String() string { return string(e) }

func (e *SessionType) UnmarshalGQL(v interface{}) error {
	return unmarshalEnum(e, v, "SessionType")
}

func (e SessionType) MarshalGQL(w io.Writer) { marshalEnum(w, e.String()) }

// FeedType describes which picture a recording shows.
type FeedType string

const (
	FeedTypeWorldFeed     FeedType = "WORLD_FEED"
	FeedTypeOnboard       FeedType = "ONBOARD"
	FeedTypePitLane       FeedType = "PIT_LANE"
	FeedTypeDataChannel   FeedType = "DATA_CHANNEL"
	FeedTypeDriverTracker FeedType = "DRIVER_TRACKER"
	FeedTypeStudio        FeedType = "STUDIO"
	FeedTypeHighlights    FeedType = "HIGHLIGHTS"
	FeedTypeDocumentary   FeedType = "DOCUMENTARY"
	FeedTypeOther         FeedType = "OTHER"
)

var AllFeedType = []FeedType{
	FeedTypeWorldFeed,
	FeedTypeOnboard,
	FeedTypePitLane,
	FeedTypeDataChannel,
	FeedTypeDriverTracker,
	FeedTypeStudio,
	FeedTypeHighlights,
	FeedTypeDocumentary,
	FeedTypeOther,
}

func (e FeedType) IsValid() bool {
	for _, v := range AllFeedType {
		if e == v {
			return true
		}
	}
	return false
}

func (e FeedType) String() string { return string(e) }

func (e *FeedType) UnmarshalGQL(v interface{}) error {
	return unmarshalEnum(e, v, "FeedType")
}

func (e FeedType) MarshalGQL(w io.Writer) { marshalEnum(w, e.String()) }

// RecordingSource is where a recording came from.
type RecordingSource string

const (
	RecordingSourceTV        RecordingSource = "TV"
	RecordingSourceStreaming RecordingSource = "STREAMING"
	RecordingSourceDVD       RecordingSource = "DVD"
	RecordingSourceBluRay    RecordingSource = "BLU_RAY"
	RecordingSourceVHS       RecordingSource = "VHS"
	RecordingSourceWeb       RecordingSource = "WEB"
	RecordingSourceOther     RecordingSource = "OTHER"
)

var AllRecordingSource = []RecordingSource{
	RecordingSourceTV,
	RecordingSourceStreaming,
	RecordingSourceDVD,
	RecordingSourceBluRay,
	RecordingSourceVHS,
	RecordingSourceWeb,
	RecordingSourceOther,
}

func (e RecordingSource) IsValid() bool {
	for _, v := range AllRecordingSource {
		if e == v {
			return true
		}
	}
	return false
}

func (e RecordingSource) String() string { return string(e) }

func (e *RecordingSource) UnmarshalGQL(v interface{}) error {
	return unmarshalEnum(e, v, "RecordingSource")
}

func (e RecordingSource) MarshalGQL(w io.Writer) { marshalEnum(w, e.String()) }

// Completeness says whether a recording covers its whole session.
type Completeness string

const (
	CompletenessComplete Completeness = "COMPLETE"
	CompletenessPartial  Completeness = "PARTIAL"
)

var AllCompleteness = []Completeness{
	CompletenessComplete,
	CompletenessPartial,
}

func (e Completeness) IsValid() bool {
	for _, v := range AllCompleteness {
		if e == v {
			return true
		}
	}
	return false
}

func (e Completeness) String() string { return string(e) }

func (e *Completeness) UnmarshalGQL(v interface{}) error {
	return unmarshalEnum(e, v, "Completeness")
}

func (e Completeness) MarshalGQL(w io.Writer) { marshalEnum(w, e.String()) }

// Criterion inputs for the sports enums. Value holds the accepted values for
// INCLUDES / EXCLUDES; IS_NULL and NOT_NULL ignore it.

type SessionTypeCriterionInput struct {
	Value    []SessionType     `json:"value"`
	Modifier CriterionModifier `json:"modifier"`
}

type FeedTypeCriterionInput struct {
	Value    []FeedType        `json:"value"`
	Modifier CriterionModifier `json:"modifier"`
}

type RecordingSourceCriterionInput struct {
	Value    []RecordingSource `json:"value"`
	Modifier CriterionModifier `json:"modifier"`
}

type CompletenessCriterionInput struct {
	Value    []Completeness    `json:"value"`
	Modifier CriterionModifier `json:"modifier"`
}
