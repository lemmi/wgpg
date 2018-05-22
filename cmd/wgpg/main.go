package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/lemmi/wgpg"
	"golang.org/x/text/language"
)

const (
	DEFAULT_PORT     = 51820
	DEFAULT_ADDR     = "127.0.0.1:8888"
	DEFAULT_ENDPOINT = "example.com:51820"
	DEFAULT_HTMLDIR  = "html/lang"
)

type config struct {
	Addr     string
	Dev      string
	WgConf   string
	Endpoint string
	Host     string
}

func (cfg config) String() string {
	buf := strings.Builder{}
	w := tabwriter.NewWriter(&buf, 0, 0, 1, ' ', 0)
	vcfg := reflect.ValueOf(cfg)
	for i := 0; i < vcfg.NumField(); i++ {
		name := vcfg.Type().Field(i).Name
		field := vcfg.Field(i).Interface()
		fmt.Fprintln(w, name, "\t", field)
	}
	w.Flush()
	return buf.String()
}

func parseConfig() config {
	var cfg config

	flag.StringVar(&cfg.Addr, "addr", DEFAULT_ADDR, "Address to bind to")
	flag.StringVar(&cfg.Dev, "dev", "", "WireGuard device")
	flag.StringVar(&cfg.WgConf, "wgconf", "", "WireGuard config path")
	flag.StringVar(&cfg.Endpoint, "endpoint", DEFAULT_ENDPOINT, "Endpoint address of the server")
	flag.Parse()

	if cfg.WgConf == "" {
		cfg.WgConf = "/etc/wireguard/" + cfg.Dev + ".conf"
	}
	if cfg.Endpoint != "" {
		cfg.Host, _, _ = net.SplitHostPort(cfg.Endpoint)
	}

	return cfg
}

type api struct {
	Cfg config
	WG  *wgpg.WG
}

func newApi(cfg config, wg *wgpg.WG) *api {
	ret := &api{
		Cfg: cfg,
		WG:  wg,
	}

	return ret
}

func (a *api) ServeAPI(w http.ResponseWriter, r *http.Request) {
	var key wgpg.Key
	keyBase64 := r.FormValue("PublicKey")
	err := key.UnmarshalText([]byte(keyBase64))
	if err != nil {
		http.Error(w, "Invalid key: "+err.Error(), http.StatusBadRequest)
		return
	}

	p, err := a.WG.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Printf("%+v\n", err)
		return
	}

	servpeer := a.WG.Interface.Peer()
	servpeer.EndPoint = a.Cfg.Endpoint
	clientconf := &wgpg.WG{
		Interface: p.Interface(DEFAULT_PORT),
		Peer: wgpg.PeerMap{
			servpeer.PublicKey: servpeer,
		},
	}

	fmt.Fprintf(w, "%s\n", clientconf)

	fmt.Printf("\n\n#Clientconf\n")
	fmt.Printf("%s\n", clientconf)

	fmt.Printf("\n\n#Serverconf\n")
	fmt.Printf("%s\n", a.WG)

	if a.Cfg.Dev != "" {
		if err = wgpg.UpdateDev(a.Cfg.Dev, p); err != nil {
			log.Println(err)
		}
	}
}

func (a *api) ServeIndex(w http.ResponseWriter, r *http.Request) {
	tpath := filepath.Join(DEFAULT_HTMLDIR, langFromPath(r.URL), "index.html")
	t := template.Must(template.ParseFiles(tpath))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	a.WG.Lock()
	defer a.WG.Unlock()
	err := t.Execute(w, a)
	if err != nil {
		log.Println(err)
	}
}

func langFromPath(u *url.URL) string {
	lang := strings.TrimLeft(u.Path, "/")
	return strings.SplitN(lang, "/", 1)[0]
}
func langRedir(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		supportedlangs, err := langSupported(DEFAULT_HTMLDIR)
		supported := language.NewMatcher(supportedlangs)
		if err != nil {
			log.Println("langSupported:", err)
		}

		if r.URL.Path == "/" {
			t, _, _ := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
			match, _, _ := supported.Match(t...)
			http.Redirect(w, r, match.String(), http.StatusFound)
		}
		lang := langFromPath(r.URL)
		_, i := language.MatchStrings(supported, lang)
		matched := supportedlangs[i].String()

		if lang != matched {
			http.Redirect(w, r, matched, http.StatusFound)
		} else {
			h(w, r)
		}
	}
}
func langSupported(dir string) ([]language.Tag, error) {
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

func main() {
	cfg := parseConfig()
	fmt.Println(cfg)

	wg, err := wgpg.LoadWG(cfg.WgConf)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", wg)

	a := newApi(cfg, wg)
	http.HandleFunc("/api", a.ServeAPI)
	http.HandleFunc("/", langRedir(a.ServeIndex))
	http.Handle("/css/", http.FileServer(http.Dir("html/")))

	if err = http.ListenAndServe(cfg.Addr, nil); err != nil {
		log.Fatal(err)
	}
}
