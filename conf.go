package main

import (
	"flag"
	"fmt"
	"net"
	"reflect"
	"strings"
	"text/tabwriter"
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
