package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/pkg/errors"
)

type api struct {
	cfg config
	wg  *WG
}

func newApi(cfg config, wg *WG) *api {
	ret := &api{
		cfg: cfg,
		wg:  wg,
	}

	return ret
}

func (a *api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var key WGKey
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
	servpeer.Endpoint = a.cfg.Endpoint
	clientconf := &WG{
		Interface: p.Interface(32, 32),
		Peer: map[WGKey]*WGPeer{
			servpeer.PublicKey: &servpeer,
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
	WG  *WG
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

func updateConfig(dev string, p *WGPeer) error {
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

	wg, err := loadWG(cfg.WgConf)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", wg)

	http.Handle("/api", newApi(cfg, wg))
	http.Handle("/", index{cfg, wg})
	http.Handle("/css/", http.FileServer(http.Dir("html/")))

	if err = http.ListenAndServe(cfg.Addr, nil); err != nil {
		log.Fatal(err)
	}
}
