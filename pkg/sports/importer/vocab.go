package importer

import "github.com/stashapp/stash/pkg/models"

// Phrases are written as they would appear in a file name; they are
// tokenised the same way as names, so "SkyF1", "Sky F1" and "sky.f1" all
// match the phrase "sky f1".

type broadcasterInfo struct {
	Name     string
	Language string
	// Ambiguous marks names shared by several channels, such as "Sky".
	Ambiguous bool
}

var broadcasterVocab = []struct {
	phrases []string
	info    broadcasterInfo
}{
	{[]string{"sky sports f1", "sky sports", "skysports", "sky f1", "sky uk"}, broadcasterInfo{Name: "Sky Sports F1", Language: "en"}},
	{[]string{"sky deutschland", "sky germany", "sky de", "skyde"}, broadcasterInfo{Name: "Sky Deutschland", Language: "de"}},
	{[]string{"sky sport italia", "sky italia", "sky it"}, broadcasterInfo{Name: "Sky Sport Italia", Language: "it"}},
	{[]string{"sky"}, broadcasterInfo{Name: "Sky Sports F1", Language: "en", Ambiguous: true}},
	{[]string{"f1 tv pro", "f1 tv", "f1tv"}, broadcasterInfo{Name: "F1 TV", Language: "en"}},
	{[]string{"rtl gp", "rtl 7", "rtl7"}, broadcasterInfo{Name: "RTL GP", Language: "nl"}},
	{[]string{"rtl nitro", "rtl"}, broadcasterInfo{Name: "RTL", Language: "de"}},
	{[]string{"servus tv", "servustv"}, broadcasterInfo{Name: "ServusTV", Language: "de"}},
	{[]string{"orf eins", "orf 1", "orf"}, broadcasterInfo{Name: "ORF", Language: "de"}},
	{[]string{"premiere world", "premiere"}, broadcasterInfo{Name: "Premiere", Language: "de"}},
	{[]string{"sf zwei", "srf zwei", "srf", "sf 2"}, broadcasterInfo{Name: "SRF", Language: "de"}},
	{[]string{"itv f1", "itv"}, broadcasterInfo{Name: "ITV", Language: "en"}},
	{[]string{"bbc one", "bbc two", "bbc"}, broadcasterInfo{Name: "BBC", Language: "en"}},
	{[]string{"channel 4", "ch 4"}, broadcasterInfo{Name: "Channel 4", Language: "en"}},
	{[]string{"espn"}, broadcasterInfo{Name: "ESPN", Language: "en"}},
	{[]string{"speed tv", "speedtv", "speed channel"}, broadcasterInfo{Name: "Speed", Language: "en"}},
	{[]string{"nbc sports", "nbcsn"}, broadcasterInfo{Name: "NBC Sports", Language: "en"}},
	{[]string{"fox sports"}, broadcasterInfo{Name: "Fox Sports", Language: "en"}},
	{[]string{"tsn"}, broadcasterInfo{Name: "TSN", Language: "en"}},
	{[]string{"rds"}, broadcasterInfo{Name: "RDS", Language: "fr"}},
	{[]string{"kayo"}, broadcasterInfo{Name: "Kayo", Language: "en"}},
	{[]string{"foxtel"}, broadcasterInfo{Name: "Foxtel", Language: "en"}},
	{[]string{"eurosport"}, broadcasterInfo{Name: "Eurosport", Language: "en"}},
	{[]string{"viaplay"}, broadcasterInfo{Name: "Viaplay", Language: "nl"}},
	{[]string{"ziggo sport", "ziggo"}, broadcasterInfo{Name: "Ziggo Sport", Language: "nl"}},
	{[]string{"canal plus", "canal"}, broadcasterInfo{Name: "Canal+", Language: "fr"}},
	{[]string{"tf 1"}, broadcasterInfo{Name: "TF1", Language: "fr"}},
	{[]string{"rai sport", "rai 1", "rai 2", "rai"}, broadcasterInfo{Name: "Rai", Language: "it"}},
	{[]string{"tv 8"}, broadcasterInfo{Name: "TV8", Language: "it"}},
	{[]string{"italia 1"}, broadcasterInfo{Name: "Italia 1", Language: "it"}},
	{[]string{"movistar f1", "movistar"}, broadcasterInfo{Name: "Movistar", Language: "es"}},
	{[]string{"dazn f1", "dazn"}, broadcasterInfo{Name: "DAZN", Language: "es"}},
	{[]string{"telecinco"}, broadcasterInfo{Name: "Telecinco", Language: "es"}},
	{[]string{"la sexta", "lasexta"}, broadcasterInfo{Name: "laSexta", Language: "es"}},
	{[]string{"antena 3"}, broadcasterInfo{Name: "Antena 3", Language: "es"}},
	{[]string{"sportv"}, broadcasterInfo{Name: "SporTV", Language: "pt"}},
	{[]string{"rede globo", "globo"}, broadcasterInfo{Name: "Globo", Language: "pt"}},
	{[]string{"fuji tv", "fujitv"}, broadcasterInfo{Name: "Fuji TV", Language: "ja"}},
	{[]string{"polsat"}, broadcasterInfo{Name: "Polsat", Language: "pl"}},
	{[]string{"m 4 sport", "m4sport"}, broadcasterInfo{Name: "M4 Sport", Language: "hu"}},
	{[]string{"mtv 3"}, broadcasterInfo{Name: "MTV3", Language: "fi"}},
	{[]string{"viasat"}, broadcasterInfo{Name: "Viasat", Language: "sv"}},
}

// languageWords are matched in any letter case. Adjectives that also name
// a Grand Prix ("German", "Italian", "Japanese") are deliberately missing.
var languageWords = map[string]string{
	"eng": "en", "english": "en", "englisch": "en",
	"ger": "de", "deu": "de", "deutsch": "de",
	"dut": "nl", "nld": "nl", "nederlands": "nl",
	"fre": "fr", "fra": "fr", "francais": "fr", "franzosisch": "fr",
	"ita": "it", "italiano": "it",
	"spa": "es", "esp": "es", "espanol": "es", "castellano": "es",
	"por": "pt", "portugues": "pt",
	"jpn": "ja",
	"pol": "pl", "polski": "pl",
	"fin": "fi", "suomi": "fi",
	"hun": "hu", "magyar": "hu",
	"swe": "sv", "svenska": "sv",
	"dan": "da",
	"nor": "no", "norsk": "no",
	"rus": "ru",
	"cze": "cs",
}

// languageCodes are only matched when written in capitals ("DE", "EN"), so
// that words such as the "de" in "Gran Premio de España" are not taken for a
// language.
var languageCodes = map[string]string{
	"en": "en", "de": "de", "nl": "nl", "fr": "fr", "it": "it", "es": "es",
	"pt": "pt", "br": "pt", "ja": "ja", "jp": "ja", "pl": "pl", "fi": "fi",
	"hu": "hu", "sv": "sv", "se": "sv", "dk": "da", "ru": "ru", "cz": "cs",
}

var seriesVocab = []struct {
	phrases []string
	name    string
}{
	{[]string{"formula 1", "formula one", "formel 1", "f 1"}, "Formula 1"},
	{[]string{"formula 2", "formel 2", "f 2"}, "Formula 2"},
	{[]string{"formula 3", "formel 3", "f 3"}, "Formula 3"},
	{[]string{"formula e"}, "Formula E"},
	{[]string{"moto gp", "motogp"}, "MotoGP"},
	{[]string{"moto 2"}, "Moto2"},
	{[]string{"moto 3"}, "Moto3"},
	{[]string{"indy car", "indycar"}, "IndyCar"},
	{[]string{"wec"}, "WEC"},
	{[]string{"dtm"}, "DTM"},
	{[]string{"nascar"}, "NASCAR"},
	{[]string{"wrc"}, "WRC"},
}

// sessionVocab lists sessions without a number, longest first where phrases
// overlap. Numbered practice and qualifying parts are handled separately.
var sessionVocab = []struct {
	phrases []string
	st      models.SessionType
	name    string
}{
	{[]string{"sprint shootout", "shootout"}, models.SessionTypeSprintQualifying, "Sprint Shootout"},
	{[]string{"sprint qualifying", "sprint quali", "sprint qualy", "sprint qualifikation"}, models.SessionTypeSprintQualifying, ""},
	{[]string{"sprint race", "sprintrennen", "sprint"}, models.SessionTypeSprint, ""},
	{[]string{"qualifying", "qualifikation", "quali", "qualy", "qualifiche", "qualifications", "clasificacion", "qualificacao"}, models.SessionTypeQualifying, ""},
	{[]string{"warm up", "warmup"}, models.SessionTypeWarmUp, ""},
	{[]string{"feature race", "main race", "race", "rennen", "course", "gara", "carrera", "corrida"}, models.SessionTypeRace, ""},
}

// practicePhrases precede or follow a practice number: "FP1", "Practice 2",
// "1. Freies Training", "EL3".
var practicePhrases = []string{"free practice", "freies training", "essais libres", "prove libere", "practice", "training", "fp", "el"}

var feedVocab = []struct {
	phrases []string
	ft      models.FeedType
}{
	{[]string{"world feed", "international feed", "int feed"}, models.FeedTypeWorldFeed},
	{[]string{"onboard", "on board", "onboards", "driver cam", "helmet cam"}, models.FeedTypeOnboard},
	{[]string{"pit lane", "pitlane", "boxengasse"}, models.FeedTypePitLane},
	{[]string{"driver tracker", "tracker"}, models.FeedTypeDriverTracker},
	{[]string{"data channel", "live timing", "timing"}, models.FeedTypeDataChannel},
	{[]string{"extended highlights", "highlights", "highlight", "zusammenfassung"}, models.FeedTypeHighlights},
	{[]string{"season review", "saisonruckblick", "saisonrueckblick", "documentary", "dokumentation", "doku"}, models.FeedTypeDocumentary},
	{[]string{"build up", "buildup", "pre race", "prerace", "pre show", "preshow", "vorbericht", "vorberichte", "post race", "postrace", "post show", "nachbericht", "analysis", "analyse", "notebook", "grid walk"}, models.FeedTypeStudio},
}

var sourceVocab = []struct {
	phrases []string
	src     models.RecordingSource
}{
	{[]string{"vhs", "vhsrip"}, models.RecordingSourceVHS},
	{[]string{"blu ray", "bluray", "bdrip", "brrip"}, models.RecordingSourceBluRay},
	{[]string{"dvdrip", "dvd 5", "dvd 9", "dvd", "video ts"}, models.RecordingSourceDVD},
	{[]string{"web dl", "webdl", "webrip", "iptv"}, models.RecordingSourceStreaming},
	{[]string{"youtube"}, models.RecordingSourceWeb},
	{[]string{"hdtv", "pdtv", "sdtv", "dvb s", "dvb t", "dvb c", "dvb", "satrip", "tvrip", "capture"}, models.RecordingSourceTV},
}

// stopWords never identify an event.
var stopWords = map[string]bool{
	"grand": true, "prix": true, "gp": true, "grosser": true, "preis": true, "gran": true, "premio": true, "grande": true,
	"premi": true, "the": true, "of": true, "von": true, "der": true, "des": true, "de": true, "du": true, "la": true,
	"le": true, "el": true, "di": true, "del": true, "do": true, "da": true, "und": true, "and": true, "live": true,
	"full": true, "complete": true, "komplett": true, "hd": true, "sd": true, "uhd": true, "fhd": true, "hevc": true,
	"avc": true, "aac": true, "mkv": true, "mp": true, "avi": true, "ts": true, "rip": true, "sat": true, "version": true,
	"edition": true, "broadcast": true, "tv": true, "sport": true, "sports": true, "channel": true, "motorsport": true,
	"motorsports": true, "formula": true, "season": true, "saison": true, "round": true, "lauf": true, "fia": true,
	"world": true, "championship": true, "wm": true, "weltmeisterschaft": true, "video": true, "vts": true, "disc": true,
	"disk": true, "cd": true, "part": true, "teil": true, "pt": true, "x": true, "h": true, "p": true, "i": true,
	"fps": true, "original": true, "audio": true, "commentary": true, "kommentar": true, "with": true, "mit": true,
	"new": true, "copy": true, "final": true, "repack": true, "proper": true, "internal": true, "rd": true, "r": true,
	"vs": true, "tag": true, "day": true, "bonus": true, "extra": true, "extras": true,
}

// eventAliases maps nicknames and translated names to words that occur in
// event names, locations, circuits or countries of the calendar.
var eventAliases = map[string][]string{
	"interlagos": {"brazil", "sao paulo"}, "brasilien": {"brazil"}, "brasil": {"brazil"}, "brazilian": {"brazil"},
	"suzuka": {"japan", "suzuka"}, "japan": {"japan"}, "japanese": {"japan"}, "fuji": {"fuji"},
	"spa": {"belgium", "spa"}, "francorchamps": {"belgium"}, "belgien": {"belgium"}, "belgique": {"belgium"}, "belgian": {"belgium"},
	"monza": {"italy", "monza"}, "italien": {"italy"}, "italia": {"italy"}, "italian": {"italy"},
	"imola": {"imola"}, "san marino": {"imola"}, "emilia": {"imola"}, "romagna": {"imola"},
	"silverstone": {"uk", "silverstone"}, "britain": {"uk"}, "british": {"uk"}, "grossbritannien": {"uk"}, "england": {"uk"},
	"brands hatch": {"brands hatch"}, "donington": {"donington"},
	"hungaroring": {"hungary"}, "ungarn": {"hungary"}, "hungarian": {"hungary"}, "budapest": {"hungary"},
	"zandvoort": {"netherlands", "zandvoort"}, "niederlande": {"netherlands"}, "holland": {"netherlands"}, "dutch": {"netherlands"}, "nederland": {"netherlands"},
	"nurburgring": {"nurburg"}, "hockenheim": {"hockenheim"}, "deutschland": {"germany"}, "german": {"germany"},
	"montreal": {"canada", "montreal"}, "kanada": {"canada"}, "canadian": {"canada"},
	"austin": {"usa", "austin"}, "cota": {"austin"}, "indianapolis": {"indianapolis"}, "watkins glen": {"watkins"},
	"usa": {"usa", "united states"}, "united states": {"united states"}, "us": {"united states"}, "american": {"united states"},
	"las vegas": {"las vegas"}, "vegas": {"las vegas"}, "miami": {"miami"},
	"mexiko": {"mexico"}, "mexican": {"mexico"},
	"baku": {"baku", "azerbaijan"}, "aserbaidschan": {"azerbaijan"},
	"jeddah": {"jeddah", "saudi"}, "dschidda": {"jeddah"}, "saudi": {"saudi"}, "saudi arabien": {"saudi"},
	"sakhir": {"sakhir", "bahrain"}, "bahrain": {"bahrain"},
	"yas marina": {"abu dhabi"}, "yas": {"abu dhabi"}, "abu dhabi": {"abu dhabi"},
	"lusail": {"qatar"}, "losail": {"qatar"}, "katar": {"qatar"},
	"marina bay": {"singapore"}, "singapur": {"singapore"},
	"melbourne": {"australia", "melbourne"}, "albert park": {"australia"}, "australien": {"australia"}, "adelaide": {"adelaide"},
	"shanghai":  {"china", "shanghai"},
	"barcelona": {"spain", "barcelona"}, "catalunya": {"spain", "catalunya"}, "spanien": {"spain"}, "espana": {"spain"}, "spanish": {"spain"},
	"jerez": {"jerez"}, "valencia": {"valencia"},
	"spielberg": {"austria", "spielberg"}, "red bull ring": {"austria"}, "a1 ring": {"austria"}, "osterreich": {"austria"}, "oesterreich": {"austria"}, "zeltweg": {"austria"}, "austrian": {"austria"},
	"styria": {"styrian"}, "steiermark": {"styrian"},
	"monte carlo": {"monaco"}, "monaco": {"monaco"},
	"paul ricard": {"france"}, "magny cours": {"france"}, "frankreich": {"france"}, "french": {"france"},
	"portimao": {"portugal"}, "estoril": {"portugal"}, "algarve": {"portugal"},
	"sepang": {"malaysia"}, "malaysian": {"malaysia"},
	"istanbul": {"turkey"}, "turkei": {"turkey"}, "turkish": {"turkey"},
	"sochi": {"russia"}, "russland": {"russia"},
	"buddh": {"india"}, "indien": {"india"},
	"yeongam": {"korea"}, "kyalami": {"south africa"}, "sudafrika": {"south africa"},
	"mugello": {"mugello"}, "tuscan": {"mugello"}, "toskana": {"mugello"},
	"sakhir outer": {"sakhir"}, "eifel": {"eifel"},
	"le mans": {"le mans"}, "sebring": {"sebring"}, "daytona": {"daytona"},
}
