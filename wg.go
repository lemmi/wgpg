package wgpg

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/crypto/curve25519"
)

type WG struct {
	sync.Mutex
	Interface Interface
	Peer      PeerMap
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
func (wg *WG) Get(k Key) (Peer, error) {
	wg.Lock()
	defer wg.Unlock()

	p, ok := wg.Peer[k]
	if ok {
		return p, nil
	}

	start, end := wg.Interface.Address.Range()
	ip, err := GetIP(start, end, len(wg.Peer)+1)
	if err != nil {
		return p, err
	}
	p = Peer{
		PublicKey:  k,
		AllowedIPs: ip.Host().IPSet(),
	}
	return wg.Peer.Set(p), nil
}

type PeerMap map[Key]Peer

func (peers *PeerMap) Set(p Peer) Peer {
	if *peers == nil {
		*peers = make(PeerMap)
	}

	(*peers)[p.PublicKey] = p
	return p
}

type Interface struct {
	Address    IP
	ListenPort int
	PrivateKey Key
	PublicKey  Key
}

func (i Interface) String() string {
	var buf strings.Builder
	fmt.Fprintln(&buf, "[Interface]")
	fmt.Fprintf(&buf, "Address = %s\n", i.Address)
	fmt.Fprintf(&buf, "ListenPort = %d\n", i.ListenPort)
	fmt.Fprintf(&buf, "PrivateKey = %s\n", i.PrivateKey)
	if i.PublicKey.IsNull() {
		fmt.Fprintf(&buf, "#PublicKey = %s\n", i.PublicKey)
	} else {
		fmt.Fprintf(&buf, "#PublicKey = %s\n", i.PrivateKey.Public())
	}
	return buf.String()
}
func (i Interface) Peer() Peer {
	return Peer{
		PublicKey:  i.PrivateKey.Public(),
		AllowedIPs: i.Address.IPSet(),
	}
}

type Peer struct {
	PublicKey           Key
	AllowedIPs          IPSet
	EndPoint            string
	PersistentKeepalive int
}

func NewPeer(k Key, ips IPSet, e string, persistentkeepalive int) Peer {
	return Peer{
		PublicKey:           k,
		AllowedIPs:          ips.Copy(),
		EndPoint:            e,
		PersistentKeepalive: persistentkeepalive,
	}
}

func (p Peer) String() string {
	var buf strings.Builder
	fmt.Fprintln(&buf, "[Peer]")
	fmt.Fprintf(&buf, "PublicKey = %s\n", p.PublicKey)
	fmt.Fprintf(&buf, "AllowedIPs = %s\n", p.AllowedIPs)
	if e := p.EndPoint; e != "" {
		fmt.Fprintf(&buf, "EndPoint = %s\n", e)
	}
	if pk := p.PersistentKeepalive; pk > 0 {
		fmt.Fprintf(&buf, "PersistentKeepalive = %d\n", p.PersistentKeepalive)
	}
	return buf.String()
}
func (p Peer) Interface(port int) Interface {
	return Interface{
		PublicKey:  p.PublicKey,
		Address:    p.AllowedIPs[0].Host(),
		ListenPort: port,
	}
}

type Key [32]byte

func (k Key) String() string {
	s, _ := k.MarshalText()
	return string(s)
}
func (k Key) MarshalText() ([]byte, error) {
	buf := make([]byte, base64.StdEncoding.EncodedLen(len(k)))
	base64.StdEncoding.Encode(buf, k[:])
	return buf, nil
}
func (k *Key) UnmarshalText(text []byte) error {
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
func (k Key) Public() Key {
	var dst [32]byte
	in := [32]byte(k)
	curve25519.ScalarBaseMult(&dst, &in)

	return Key(dst)
}
func (k Key) IsNull() bool {
	return k == Key{}
}

const (
	WG_SECTION_NONE = iota
	WG_SECTION_INTERFACE
	WG_SECTION_PEER
)

func LoadWG(path string) (*WG, error) {
	wg := new(WG)
	wg.Peer = make(PeerMap)
	var err error

	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "Can't open wg-config")
	}
	defer f.Close()

	var section int
	var linenumber int
	var peerbuild Peer
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
			if !peerbuild.PublicKey.IsNull() {
				wg.Peer.Set(peerbuild)
			}
			peerbuild = Peer{}
			continue
		}
		if section == WG_SECTION_NONE {
			continue
		}

		s := strings.SplitN(l, "=", 2)
		if len(s) != 2 {
			return nil, errors.Errorf("Expected assignment, got %q", s)
		}

		field := strings.ToLower(strings.TrimSpace(s[0]))
		value := strings.TrimSpace(s[1])

		switch section {
		case WG_SECTION_NONE:
			err = errors.Errorf("No section specified")
		case WG_SECTION_INTERFACE:
			I := &wg.Interface
			switch field {
			case "address":
				err = I.Address.UnmarshalText([]byte(value))
			case "privatekey":
				err = I.PrivateKey.UnmarshalText([]byte(value))
			case "listenport":
				I.ListenPort, err = strconv.Atoi(value)
			default:
				err = errors.Errorf("Invalid field %q", s[0])
			}
		case WG_SECTION_PEER:
			switch field {
			case "allowedips":
				err = peerbuild.AllowedIPs.UnmarshalText([]byte(value))
			case "publickey":
				err = peerbuild.PublicKey.UnmarshalText([]byte(value))
			case "endpoint":
				peerbuild.EndPoint = value
			case "persistentkeepalive":
				peerbuild.PersistentKeepalive, err = strconv.Atoi(value)
			default:
				err = errors.Errorf("Invalid field %q", s[0])
			}
		}

		if err != nil {
			return nil, errors.Wrapf(err, "ERROR: Line %d", linenumber)
		}
	}

	if !peerbuild.PublicKey.IsNull() {
		wg.Peer.Set(peerbuild)
	}

	return wg, nil
}
