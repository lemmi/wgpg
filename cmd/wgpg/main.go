package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/lemmi/wgpg"
	"github.com/pkg/errors"
)

const (
	DEFAULT_PORT     = 51820
	DEFAULT_ADDR     = "127.0.0.1:8888"
	DEFAULT_ENDPOINT = "example.com:51820"
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
	cfg config
	wg  *wgpg.WG
}

func newApi(cfg config, wg *wgpg.WG) *api {
	ret := &api{
		cfg: cfg,
		wg:  wg,
	}

	return ret
}

func (a *api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var key wgpg.Key
	keyBase64 := r.FormValue("PublicKey")
	err := key.UnmarshalText([]byte(keyBase64))
	if err != nil {
		http.Error(w, "Invalid key: "+err.Error(), http.StatusBadRequest)
		return
	}

	p, err := a.wg.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	servpeer := a.wg.Interface.Peer()
	servpeer.EndPoint = a.cfg.Endpoint
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
	fmt.Printf("%s\n", a.wg)

	if a.cfg.Dev != "" {
		if err = updateConfig(a.cfg.Dev, p); err != nil {
			log.Println(err)
		}
	}
}

type index struct {
	Cfg config
	WG  *wgpg.WG
}

func (i index) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.ParseFiles("html/index.html"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	i.WG.Lock()
	defer i.WG.Unlock()
	err := t.Execute(w, i)
	if err != nil {
		log.Println(err)
	}
}

func updateConfig(dev string, p wgpg.Peer) error {
	f, err := ioutil.TempFile("/tmp", "")
	if err != nil {
		return errors.Wrap(err, "Cannot write config update")
	}

	fname := f.Name()
	defer os.Remove(fname)

	_, err = f.WriteString(p.String())
	f.Close()
	if err != nil {
		return errors.Wrap(err, "Cannot write config update")
	}

	cmd := exec.Command("wg", "addconf", dev, fname)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	cfg := parseConfig()
	fmt.Println(cfg)

	wg, err := wgpg.LoadWG(cfg.WgConf)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", wg)
	fmt.Printf("%x\n", wg.Interface.Address.Addr)

	http.Handle("/api", newApi(cfg, wg))
	http.Handle("/", index{cfg, wg})
	http.Handle("/css/", http.FileServer(http.Dir("html/")))

	if err = http.ListenAndServe(cfg.Addr, nil); err != nil {
		log.Fatal(err)
	}
}
