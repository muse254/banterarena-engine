package attribution

import (
	"strings"
	"unicode"
)

// Country is one side that can appear in a banter war.
type Country struct {
	Code string
	// Terms are lowercase names, demonyms, cities and slang that signal the
	// country in a tweet or a profile location. Matching is whole-word.
	// Add yours via PR: this list is deliberately open.
	Terms []string
}

// Gazetteer is the built-in list. Keep terms specific: a term that appears in
// another country's vocabulary will make locations ambiguous (and ambiguous
// locations are ignored, never guessed).
var Gazetteer = []Country{
	{"KE", []string{"kenya", "kenyan", "kenyans", "nairobi", "mombasa", "kisumu", "nakuru", "eldoret", "kot"}},
	{"ZA", []string{"south africa", "south african", "south africans", "mzansi", "johannesburg", "joburg", "jozi", "cape town", "durban", "pretoria", "soweto"}},
	{"NG", []string{"nigeria", "nigerian", "nigerians", "naija", "lagos", "abuja", "kano", "ibadan", "port harcourt"}},
	{"GH", []string{"ghana", "ghanaian", "ghanaians", "accra", "kumasi"}},
	{"UG", []string{"uganda", "ugandan", "ugandans", "kampala", "entebbe"}},
	{"TZ", []string{"tanzania", "tanzanian", "tanzanians", "dar es salaam", "dodoma", "arusha", "zanzibar"}},
}

// normalize lowercases and turns everything that isn't a letter or digit into
// a single space, padded so " term " matches whole words and phrases.
func normalize(s string) string {
	var b strings.Builder
	b.WriteByte(' ')
	space := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	if !space {
		b.WriteByte(' ')
	}
	return b.String()
}

// flagCodes finds country flag emoji (pairs of regional indicators) and
// returns their ISO codes.
func flagCodes(s string) []string {
	var out []string
	rs := []rune(s)
	for i := 0; i+1 < len(rs); i++ {
		a, b := rs[i], rs[i+1]
		if a >= 0x1F1E6 && a <= 0x1F1FF && b >= 0x1F1E6 && b <= 0x1F1FF {
			out = append(out, string([]rune{'A' + (a - 0x1F1E6), 'A' + (b - 0x1F1E6)}))
			i++
		}
	}
	return out
}

// Mentions returns the countries signalled in text, by flag emoji or gazetteer term.
func Mentions(text string) map[string]bool {
	n := normalize(text)
	out := map[string]bool{}
	for _, c := range Gazetteer {
		for _, t := range c.Terms {
			if strings.Contains(n, " "+t+" ") {
				out[c.Code] = true
				break
			}
		}
	}
	for _, code := range flagCodes(text) {
		for _, c := range Gazetteer {
			if c.Code == code {
				out[code] = true
			}
		}
	}
	return out
}

// ResolveLocation maps a free-text profile location ("Nairobi, Kenya", "Lagos 🇳🇬")
// to one country. Anything unclear (empty, unknown, or naming two countries)
// returns ok=false: locations are self-reported and spoofable, so we never guess.
func ResolveLocation(loc string) (code string, ok bool) {
	m := Mentions(loc)
	if len(m) != 1 {
		return "", false
	}
	for c := range m {
		return c, true
	}
	return "", false
}
