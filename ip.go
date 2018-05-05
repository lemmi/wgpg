package wgpg

import (
	"bytes"
	"encoding/binary"
	"net"

	"github.com/pkg/errors"
)

type netIP [16]byte
type netIPMask [16]byte

func (ip netIP) IP() net.IP {
	return net.IP(ip[:])
}

type IP struct {
	IP  net.IP
	Net *net.IPNet
}

func (i IP) String() string {
	return (&net.IPNet{
		IP:   i.IP,
		Mask: i.Net.Mask,
	}).String()
}
func (i *IP) UnmarshalText(text []byte) error {
	var err error
	i.IP, i.Net, err = net.ParseCIDR(string(bytes.TrimSpace(text)))
	return err
}
func (i IP) Copy() IP {
	return IP{
		IP: append(net.IP{}, i.IP...),
		Net: &net.IPNet{
			IP:   append(net.IP{}, i.Net.IP...),
			Mask: append(net.IPMask{}, i.Net.Mask...),
		},
	}
}
func (i IP) IPSet() IPSet {
	return IPSet{i.Copy()}
}
func (i IP) Range() (start net.IP, end net.IP) {
	ip, _ := ipToUInt(i.IP)
	mask, _ := ipToUInt(net.IP(i.Net.Mask))
	s := ip & mask
	e := ip | ^mask
	start = make(net.IP, 4)
	end = make(net.IP, 4)

	binary.BigEndian.PutUint32([]byte(start), s)
	binary.BigEndian.PutUint32([]byte(end), e)

	return start, end
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

	return nil
}

func ipToUInt(ip net.IP) (uint32, error) {
	v4 := ip.To4()
	if v4 == nil {
		return 0, errors.New("Invalid address")
	}
	return binary.BigEndian.Uint32(v4), nil
}

func GetIP(start, end net.IP, n int) (ip net.IP, err error) {
	var sip, eip uint32
	if sip, err = ipToUInt(start); err != nil {
		return nil, err
	}
	if eip, err = ipToUInt(end); err != nil {
		return nil, err
	}
	nip := sip + uint32(n)
	if sip+uint32(n) >= eip {
		return nil, errors.Errorf("Out of Addresses: %d < %d < %d", sip, nip, eip)
	}
	t := make([]byte, 4)
	binary.BigEndian.PutUint32(t[:], nip)
	newip := net.IP(t[:])
	return newip, nil
}
func (i IPSet) Copy() IPSet {
	ret := make(IPSet, 0, len(i))
	for _, ip := range i {
		ret = append(ret, ip)
	}
	return ret
}
