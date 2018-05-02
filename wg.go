package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/crypto/curve25519"
)

type WG struct {
	sync.Mutex
	Interface WGInterface
	Peer      map[WGKey]*WGPeer
}

func (wg *WG) String() string {
	wg.Lock()
	defer wg.Unlock()
	var buf strings.Builder
	buf.WriteString(wg.Interface.String())
	for _, p := range wg.Peer {
		buf.WriteString("\n")
		buf.WriteString(p.String())
	}
	return buf.String()
}
func (wg *WG) Get(k WGKey) (*WGPeer, error) {
	wg.Lock()
	defer wg.Unlock()

	if wg.Peer == nil {
		wg.Peer = make(map[WGKey]*WGPeer)
	}

	p, ok := wg.Peer[k]
	if ok {
		return p, nil
	}

	ip, err := GetIP(wg.Interface.Address, len(wg.Peer))
	if err != nil {
		return nil, err
	}

	p = &WGPeer{
		PublicKey: k,
		AllowedIPs: IPSet{
			IP{
				IP: ip,
				Net: &net.IPNet{
					IP:   ip,
					Mask: net.CIDRMask(32, 32),
				},
			},
		},
	}
	wg.Peer[k] = p

	return p, nil
}

type WGInterface struct {
	Address    IP
	ListenPort int
	PrivateKey WGKey
	PublicKey  WGKey
}

func (i WGInterface) String() string {
	var buf strings.Builder
	fmt.Fprintln(&buf, "[Interface]")
	fmt.Fprintf(&buf, "Address = %s\n", i.Address)
	fmt.Fprintf(&buf, "ListenPort = %d\n", i.ListenPort)
	fmt.Fprintf(&buf, "PrivateKey = %s\n", i.PrivateKey)
	if (WGKey{}) != i.PublicKey {
		fmt.Fprintf(&buf, "#PublicKey = %s\n", i.PublicKey)
	} else {
		fmt.Fprintf(&buf, "#PublicKey = %s\n", i.PrivateKey.Public())
	}
	return buf.String()
}
func (i WGInterface) Peer() WGPeer {
	return WGPeer{
		PublicKey:  i.PrivateKey.Public(),
		AllowedIPs: IPSet{i.Address.Copy()},
	}
}

type WGPeer struct {
	PublicKey           WGKey
	AllowedIPs          IPSet
	Endpoint            string
	PersistentKeepalive int
}

func (p WGPeer) String() string {
	var buf strings.Builder
	fmt.Fprintln(&buf, "[Peer]")
	fmt.Fprintf(&buf, "PublicKey = %s\n", p.PublicKey)
	fmt.Fprintf(&buf, "AllowedIPs = %s\n", p.AllowedIPs)
	if e := p.Endpoint; e != "" {
		fmt.Fprintf(&buf, "EndPoint = %s\n", e)
	}
	if pk := p.PersistentKeepalive; pk > 0 {
		fmt.Fprintf(&buf, "PersistentKeepalive = %d\n", p.PersistentKeepalive)
	}
	return buf.String()
}
func (p WGPeer) Interface(ones, bits int) WGInterface {
	addr := p.AllowedIPs[0].Copy()
	addr.Net = &net.IPNet{
		IP:   append(net.IP{}, addr.IP...),
		Mask: net.CIDRMask(ones, bits),
	}
	return WGInterface{
		PublicKey:  p.PublicKey,
		Address:    p.AllowedIPs[0].Copy(),
		ListenPort: DEFAULT_PORT,
	}
}

type WGKey [32]byte

func (k WGKey) String() string {
	s, _ := k.MarshalText()
	return string(s)
}
func (k WGKey) MarshalText() ([]byte, error) {
	buf := make([]byte, base64.StdEncoding.EncodedLen(len(k)))
	base64.StdEncoding.Encode(buf, k[:])
	return buf, nil
}
func (k *WGKey) UnmarshalText(text []byte) error {
	buf := make([]byte, 36)
	n, err := base64.StdEncoding.Decode(buf, text)
	if err != nil {
		return err
	}
	if n != 32 {
		return fmt.Errorf("Invalid keylength")
	}
	copy(k[:], buf)
	return nil
}
func (k WGKey) Public() WGKey {
	var dst [32]byte
	in := [32]byte(k)
	curve25519.ScalarBaseMult(&dst, &in)

	return WGKey(dst)
}

const (
	WG_SECTION_NONE = iota
	WG_SECTION_INTERFACE
	WG_SECTION_PEER
)

func loadWG(path string) (*WG, error) {
	wg := new(WG)
	wg.Peer = make(map[WGKey]*WGPeer)
	var err error

	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "Can't open wg-config")
	}
	defer f.Close()

	var section int
	var linenumber int
	var peerbuild *WGPeer
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		linenumber++
		l := scan.Text()
		l = strings.TrimSpace(l)
		if idx := strings.Index(l, "#"); idx >= 0 {
			l = l[0:idx]
		}
		if len(l) == 0 {
			continue
		}
		switch l {
		case "[Interface]":
			section = WG_SECTION_INTERFACE
			continue
		case "[Peer]":
			section = WG_SECTION_PEER
			peerbuild = new(WGPeer)
			continue
		}

		s := strings.SplitN(l, "=", 2)
		if len(s) != 2 {
			return nil, errors.Errorf("Expected assignment, got %q", s)
		}

		field := strings.ToLower(strings.TrimSpace(s[0]))
		s[1] = strings.TrimSpace(s[1])

		switch section {
		case WG_SECTION_NONE:
			err = errors.Errorf("No section specified")
		case WG_SECTION_INTERFACE:
			I := &wg.Interface
			switch field {
			case "address":
				err = I.Address.UnmarshalText([]byte(s[1]))
			case "privatekey":
				err = I.PrivateKey.UnmarshalText([]byte(s[1]))
			case "listenport":
				I.ListenPort, err = strconv.Atoi(s[1])
			default:
				err = errors.Errorf("Invalid field %q", s[0])
			}
		case WG_SECTION_PEER:
			switch field {
			case "allowedips":
				err = peerbuild.AllowedIPs.UnmarshalText([]byte(s[1]))
			case "publickey":
				err = peerbuild.PublicKey.UnmarshalText([]byte(s[1]))
				if err == nil {
					wg.Peer[peerbuild.PublicKey] = peerbuild
				}
			case "endpoint":
				peerbuild.Endpoint = s[1]
			case "persistentkeepalive":
				peerbuild.PersistentKeepalive, err = strconv.Atoi(s[1])
			default:
				err = errors.Errorf("Invalid field %q", s[0])
			}
		}

		if err != nil {
			return nil, errors.Wrapf(err, "ERROR: Line %d", linenumber)
		}
	}

	return wg, nil
}
