package lang

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/text/language"
)

func FromPath(u *url.URL) string {
	lang := strings.TrimLeft(u.Path, "/")
	return strings.SplitN(lang, "/", 1)[0]
}
func Redir(h http.HandlerFunc, search string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		supportedlangs, err := Supported(search)
		supported := language.NewMatcher(supportedlangs)
		if err != nil {
			log.Println("langSupported:", err)
		}

		if r.URL.Path == "/" {
			t, _, _ := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
			match, _, _ := supported.Match(t...)
			http.Redirect(w, r, match.String(), http.StatusFound)
		}
		lang := FromPath(r.URL)
		_, i := language.MatchStrings(supported, lang)
		matched := supportedlangs[i].String()

		if lang != matched {
			http.Redirect(w, r, matched, http.StatusFound)
		} else {
			h(w, r)
		}
	}
}
func Supported(dir string) ([]language.Tag, error) {
	var langs []language.Tag

	dirs, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		tag, err := language.Parse(dir.Name())
		if err != nil {
			return nil, err
		}
		if tag == language.English {
			langs = append([]language.Tag{tag}, langs...)
		} else {
			langs = append(langs, tag)
		}
	}
	return langs, nil
}
