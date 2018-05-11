package wgpg

import (
	"bytes"
	"math/big"
	"net"
	"sort"

	"github.com/pkg/errors"
)

var ip4in6prefix = [12]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}

type IP struct {
	Addr      [16]byte
	AddrLen   uint8
	PrefixLen uint8
}

func (i *IP) UnmarshalText(text []byte) error {
	var err error
	ip, ipnet, err := net.ParseCIDR(string(bytes.TrimSpace(text)))
	if err != nil {
		return err
	}

	ones, bits := ipnet.Mask.Size()
	if ones == 0 && bits == 0 {
		return errors.Errorf("Bad netmask: ones: %d, bits: %d", ones, bits)
	}

	i.AddrLen, i.PrefixLen = uint8(bits), uint8(ones)
	*i = i.setBytes(ip)
	return err
}
func (i IP) NetIP() net.IP {
	if bytes.HasPrefix(i.Addr[:], ip4in6prefix[:]) {
		return net.IP(i.Addr[12:])
	} else {
		return net.IP(i.Addr[16-i.AddrLen/8:])
	}
}
func (i IP) NetMask() net.IPMask {
	return net.CIDRMask(int(i.PrefixLen), int(i.AddrLen))
}
func (i IP) Network() IP {
	network := i.NetIP().Mask(i.NetMask())
	copy(i.Addr[:], network)
	return i
}
func (i IP) String() string {
	return (&net.IPNet{
		IP:   i.NetIP(),
		Mask: i.NetMask(),
	}).String()
}
func (i IP) Contains(ip IP) bool {
	ipnet := net.IPNet{
		IP:   i.NetIP(),
		Mask: i.NetMask(),
	}
	return ipnet.Contains(ip.NetIP())
}
func (i IP) IPSet() IPSet {
	return IPSet{i}
}
func (i IP) Range() (IP, IP) {
	start := i.Network()
	end := i.Network()
	var mask [16]byte
	copy(mask[16-start.AddrLen/8:], end.NetMask())
	for k := range end.Addr {
		end.Addr[k] |= ^mask[k]
	}
	return start, end
}
func (i IP) Host() IP {
	i.PrefixLen = i.AddrLen
	return i
}
func (i IP) BigInt() *big.Int {
	return new(big.Int).SetBytes(i.NetIP())
}
func (i IP) setBytes(b []byte) IP {
	copy(i.Addr[16-len(b):], b)
	return i
}

type IPSet []IP

func (i IPSet) String() string {
	var buf bytes.Buffer
	var sep string

	for _, ip := range i {
		buf.WriteString(sep)
		buf.WriteString(ip.String())
		sep = ", "
	}

	return buf.String()
}
func (i *IPSet) UnmarshalText(text []byte) error {
	s := bytes.Split(text, []byte{','})
	for _, iptext := range s {
		var ip IP
		if err := ip.UnmarshalText(iptext); err != nil {
			return err
		}

		*i = append(*i, ip)
	}

	i.Sort()

	return nil
}
func (is IPSet) Sort() IPSet {
	sort.Slice(is, is.Less)
	return is
}
func (is IPSet) Less(i, j int) bool {
	if is[i].AddrLen-is[i].PrefixLen < is[j].AddrLen-is[j].PrefixLen {
		return true
	}
	return bytes.Compare(is[i].Addr[:], is[j].Addr[:]) == -1
}

func GetIP(start, end IP, n int) (ip IP, err error) {
	sip := start.BigInt()
	eip := end.BigInt()
	nip := new(big.Int)
	nip.Add(sip, big.NewInt(int64(n)))
	if nip.Cmp(eip) != -1 {
		return IP{}, errors.Errorf("Out of Addresses: %d < %d < %d", sip, nip, eip)
	}
	return start.setBytes(nip.Bytes()), nil
}
func (i IPSet) Copy() IPSet {
	ret := make(IPSet, 0, len(i))
	for _, ip := range i {
		ret = append(ret, ip)
	}
	return ret
}
