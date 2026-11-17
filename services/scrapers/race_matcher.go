package scrapers

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// discoveredRace is one row of the F1.com season results index.
type discoveredRace struct {
	name     string
	slug     string
	date     string    // raw text from the index, e.g. "23 Aug"
	raceDate time.Time // parsed race day (UTC midnight); zero if it could not be parsed
	url      string
}

type matchMethod string

const (
	matchedByDate    matchMethod = "date"
	matchedByName    matchMethod = "name"
	matchedByCountry matchMethod = "country"
)

const (
	// F1.com lists the race day (Sunday); our DB start date is usually the
	// Friday. We accept a window [start, end] widened by this many days each side.
	dateToleranceDays = 1
	// If RaceEndDate is missing or unusable, assume a Fri -> Sun weekend.
	defaultWeekendDays = 2
	maxWeekendDays     = 7
)

var indexDatePattern = regexp.MustCompile(`(\d{1,2})\s+([A-Za-z]{3})`)

// parseIndexDate turns "23 Aug" (or "21 - 23 Aug", "3 Aug") into a UTC date in
// the given season year. For ranges it returns the last date in the text.
func parseIndexDate(text string, year int) (time.Time, bool) {
	m := indexDatePattern.FindStringSubmatch(text)
	if m == nil {
		return time.Time{}, false
	}
	t, err := time.Parse("2 Jan 2006", fmt.Sprintf("%s %s %d", m[1], m[2], year))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// raceWindow returns the first and last day of the race weekend.
func raceWindow(race *Race) (from, to time.Time, ok bool) {
	if race.RaceStartDate.IsZero() {
		return time.Time{}, time.Time{}, false
	}
	from = dateOnly(race.RaceStartDate)
	to = from.AddDate(0, 0, defaultWeekendDays)
	if !race.RaceEndDate.IsZero() {
		end := dateOnly(race.RaceEndDate)
		if end.After(from) && end.Sub(from) <= maxWeekendDays*24*time.Hour {
			to = end
		}
	}
	return from, to, true
}

func isUsed(used map[string]struct{}, d discoveredRace) bool {
	if used == nil {
		return false
	}
	_, taken := used[d.url]
	return taken
}

// matchRace tries, in order: date window, race name, country.
// `used` holds result-page URLs already claimed by another race this run, so
// two races can never be linked to the same page.
func matchRace(discovered []discoveredRace, race *Race, used map[string]struct{}) (discoveredRace, matchMethod, bool) {
	if d, ok := matchByDate(discovered, race, used); ok {
		return d, matchedByDate, true
	}
	if d, ok := matchByText(discovered, normalizeText(race.Name), used); ok {
		return d, matchedByName, true
	}
	if d, ok := matchByText(discovered, normalizeText(race.Country), used); ok {
		return d, matchedByCountry, true
	}
	return discoveredRace{}, "", false
}

// matchByDate picks the index entry whose race day falls inside the race's
// weekend window (+/- tolerance). If several qualify it takes the one closest
// to the expected race day; an exact tie is treated as ambiguous (no match).
func matchByDate(discovered []discoveredRace, race *Race, used map[string]struct{}) (discoveredRace, bool) {
	from, to, ok := raceWindow(race)
	if !ok {
		return discoveredRace{}, false
	}
	lo := from.AddDate(0, 0, -dateToleranceDays)
	hi := to.AddDate(0, 0, dateToleranceDays)

	var best discoveredRace
	bestDist := time.Duration(-1)
	tie := false

	for _, d := range discovered {
		if d.raceDate.IsZero() || isUsed(used, d) {
			continue
		}
		day := dateOnly(d.raceDate)
		if day.Before(lo) || day.After(hi) {
			continue
		}
		dist := day.Sub(to)
		if dist < 0 {
			dist = -dist
		}
		switch {
		case bestDist < 0 || dist < bestDist:
			best, bestDist, tie = d, dist, false
		case dist == bestDist:
			tie = true
		}
	}
	if bestDist < 0 || tie {
		return discoveredRace{}, false
	}
	return best, true
}

// slugAliases maps an F1.com URL slug to other words that appear in official
// race names (demonyms, local-language names, circuits, cities). Keys and
// values are normalized by normalizeText, so accents and case don't matter.
// Add a line here whenever a new race shows up in the "not found" logs.
var slugAliases = map[string][]string{
	"australia":      {"australian", "melbourne"},
	"china":          {"chinese", "shanghai"},
	"japan":          {"japanese", "suzuka"},
	"bahrain":        {"sakhir"},
	"saudi-arabia":   {"saudi", "saudi arabian", "jeddah"},
	"miami":          {},
	"canada":         {"canadian", "montreal"},
	"monaco":         {"monte carlo"},
	"spain":          {"espana", "spanish", "espanol", "barcelona", "catalunya", "madrid"},
	"austria":        {"austrian", "osterreich", "spielberg"},
	"great-britain":  {"british", "silverstone"},
	"belgium":        {"belgian", "spa francorchamps"},
	"hungary":        {"hungarian", "magyar", "budapest"},
	"netherlands":    {"dutch", "nederland", "holland", "zandvoort"},
	"italy":          {"italia", "italian", "monza"},
	"azerbaijan":     {"baku"},
	"singapore":      {"marina bay"},
	"united-states":  {"usa", "us", "americas", "austin"},
	"mexico":         {"mexican", "mexico city", "ciudad de mexico"},
	"brazil":         {"brasil", "brazilian", "sao paulo", "interlagos"},
	"las-vegas":      {"vegas"},
	"qatar":          {"lusail"},
	"abu-dhabi":      {"yas marina"},
	"emilia-romagna": {"emilia", "imola"},
	"portugal":       {"portuguese", "portimao"},
	"turkey":         {"turkish", "turkiye", "istanbul"},
	"france":         {"french"},
	"germany":        {"german", "deutschland"},
	"russia":         {"russian", "sochi"},
}

// keysForSlug returns every normalized phrase that identifies this slug.
func keysForSlug(slug string) []string {
	keys := []string{normalizeText(strings.ReplaceAll(slug, "-", " "))}
	for _, alias := range slugAliases[slug] {
		keys = append(keys, normalizeText(alias))
	}
	return keys
}

// matchByText finds the single index entry whose slug (or one of its aliases)
// appears as a whole phrase in text. Whole-phrase matching avoids false
// positives like "oman" inside "romania". If more than one entry matches, the
// one with the longest matching key wins; if that is still a tie, we refuse to
// guess and return no match.
func matchByText(discovered []discoveredRace, text string, used map[string]struct{}) (discoveredRace, bool) {
	if text == "" {
		return discoveredRace{}, false
	}
	haystack := " " + text + " "

	var best discoveredRace
	bestLen := 0
	tie := false

	for _, d := range discovered {
		if d.slug == "" || isUsed(used, d) {
			continue
		}
		longest := 0
		for _, key := range keysForSlug(d.slug) {
			if key == "" {
				continue
			}
			if strings.Contains(haystack, " "+key+" ") && len(key) > longest {
				longest = len(key)
			}
		}
		if longest == 0 {
			continue
		}
		switch {
		case longest > bestLen:
			best, bestLen, tie = d, longest, false
		case longest == bestLen:
			tie = true
		}
	}
	if bestLen == 0 || tie {
		return discoveredRace{}, false
	}
	return best, true
}

var diacriticReplacer = strings.NewReplacer(
	"à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a", "å", "a", "ā", "a",
	"ç", "c", "č", "c", "ć", "c",
	"è", "e", "é", "e", "ê", "e", "ë", "e", "ē", "e", "ě", "e",
	"ì", "i", "í", "i", "î", "i", "ï", "i", "ī", "i",
	"ñ", "n", "ń", "n", "ň", "n",
	"ò", "o", "ó", "o", "ô", "o", "õ", "o", "ö", "o", "ō", "o", "ő", "o",
	"ù", "u", "ú", "u", "û", "u", "ü", "u", "ū", "u", "ů", "u", "ű", "u",
	"ý", "y", "ÿ", "y",
	"š", "s", "ś", "s", "ž", "z", "ź", "z", "ż", "z", "ł", "l",
	"ø", "o", "æ", "ae", "œ", "oe", "ß", "ss",
)

var nonAlnumPattern = regexp.MustCompile(`[^a-z0-9]+`)

// normalizeText lowercases, strips accents (both precomposed and combining),
// and collapses everything that isn't a letter or digit into single spaces:
//
//	"GRAN PREMIO DE ESPAÑA 2026"  -> "gran premio de espana 2026"
//	"SÃO PAULO"                   -> "sao paulo"
//	"PREMIO D'ITALIA"             -> "premio d italia"
func normalizeText(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, s)
	s = diacriticReplacer.Replace(s)
	s = nonAlnumPattern.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
