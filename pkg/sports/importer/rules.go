package importer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Rule is a user-defined naming template such as
// "{series}.{year}.R{round}.{event}.{session}.{broadcaster}". Placeholders:
// {year} {round} {part} {series} {event} {session} {broadcaster} {lang}
// {feed} {driver} and {*} for anything to ignore. Spaces, dots, dashes and
// underscores in the template match any run of those separators. Matching
// ignores letter case and covers the whole file name without extension.
type Rule struct {
	Template string
	re       *regexp.Regexp
	fields   []string
}

var placeholder = regexp.MustCompile(`\{([a-z*]+)\}`)

var placeholderPatterns = map[string]string{
	"year":        `(19[5-9]\d|20\d\d)`,
	"round":       `(\d{1,2})`,
	"part":        `(\d{1,2})`,
	"series":      `(.+?)`,
	"event":       `(.+?)`,
	"session":     `(.+?)`,
	"broadcaster": `(.+?)`,
	"lang":        `([A-Za-z]{2,10})`,
	"feed":        `(.+?)`,
	"driver":      `(.+?)`,
	"*":           `(.*?)`,
}

const separatorClass = `[\s._\-]+`

func quoteLiteral(s string) string {
	var b strings.Builder
	inSep := false
	for _, r := range s {
		if r == ' ' || r == '.' || r == '_' || r == '-' {
			if !inSep {
				b.WriteString(separatorClass)
				inSep = true
			}
			continue
		}
		inSep = false
		b.WriteString(regexp.QuoteMeta(string(r)))
	}
	return b.String()
}

// CompileRule parses a naming template.
func CompileRule(template string) (*Rule, error) {
	t := strings.TrimSpace(template)
	if t == "" {
		return nil, fmt.Errorf("empty naming rule")
	}

	var b strings.Builder
	b.WriteString(`(?i)^`)
	var fields []string
	last := 0
	for _, m := range placeholder.FindAllStringSubmatchIndex(t, -1) {
		b.WriteString(quoteLiteral(t[last:m[0]]))
		name := t[m[2]:m[3]]
		pat, ok := placeholderPatterns[name]
		if !ok {
			return nil, fmt.Errorf("naming rule %q: unknown placeholder {%s}", template, name)
		}
		b.WriteString(pat)
		fields = append(fields, name)
		last = m[1]
	}
	b.WriteString(quoteLiteral(t[last:]))
	b.WriteString(`$`)

	if len(fields) == 0 {
		return nil, fmt.Errorf("naming rule %q has no placeholders", template)
	}

	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, fmt.Errorf("naming rule %q: %w", template, err)
	}
	return &Rule{Template: t, re: re, fields: fields}, nil
}

// CompileRules compiles templates, returning the valid rules and an error
// describing the invalid ones.
func CompileRules(templates []string) ([]*Rule, error) {
	var ret []*Rule
	var errs []string
	for _, t := range templates {
		r, err := CompileRule(t)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		ret = append(ret, r)
	}
	if len(errs) > 0 {
		return ret, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return ret, nil
}

// apply fills p from name if the rule matches. Values a rule captures are
// read with the same vocabulary as names, but count as explicit.
func (r *Rule) apply(name string, p *Parsed) bool {
	m := r.re.FindStringSubmatch(name)
	if m == nil {
		return false
	}

	for i, field := range r.fields {
		v := strings.TrimSpace(m[i+1])
		if v == "" {
			continue
		}
		switch field {
		case "year", "round", "part":
			n, err := strconv.Atoi(v)
			if err != nil {
				continue
			}
			switch field {
			case "year":
				p.Year = n
			case "round":
				p.Round = n
			case "part":
				p.Part = n
			}
			p.setOrigin(field, OriginRule)
		case "event":
			toks := tokenise(v)
			p.EventWords = eventWords(toks)
		case "driver":
			p.FeedSubject = v
			if len(v) == 3 {
				p.FeedSubject = strings.ToUpper(v)
			}
			if p.Feed == "" {
				p.Feed = "ONBOARD"
				p.setOrigin("feed", OriginRule)
			}
		case "lang":
			toks := tokenise(v)
			if len(toks) == 1 {
				if l, ok := languageWords[toks[0].text]; ok {
					p.Language = l
				} else if l, ok := languageCodes[toks[0].text]; ok {
					p.Language = l
				} else {
					p.Language = strings.ToLower(v)
				}
				p.setOrigin("language", OriginRule)
			}
		case "series", "session", "broadcaster", "feed":
			before := *p
			toks := tokenise(v)
			p.extract(toks, OriginRule)
			if field == "broadcaster" && p.Broadcaster == before.Broadcaster {
				// a broadcaster the vocabulary does not know
				p.Broadcaster = v
				p.setOrigin("broadcaster", OriginRule)
			}
		}
	}
	return true
}
