package main

import (
	"fmt"
	"log"
	"net/http"
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

	clientconf := &WG{
		Interface: p.Interface(),
		Peer:      []WGPeer{a.wg.Interface.Peer()},
	}
	clientconf.Peer[0].AllowedIPs = IPSet{a.wg.Interface.Address}
	clientconf.Peer[0].Endpoint = a.cfg.Endpoint

	fmt.Fprintf(w, "%s\n", clientconf)

	fmt.Printf("\n\n#Clientconf\n")
	fmt.Printf("%s\n", clientconf)

	fmt.Printf("\n\n#Serverconf\n")
	fmt.Printf("%s\n", a.wg)

}

func main() {
	cfg := parseConfig()
	fmt.Println(cfg)

	wg, err := loadWG("/etc/wireguard/wg0.conf")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", wg)

	http.Handle("/api", newApi(cfg, wg))
	http.Handle("/", http.FileServer(http.Dir("html/")))

	if err = http.ListenAndServe(cfg.Addr, nil); err != nil {
		log.Fatal(err)
	}
}
