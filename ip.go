package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
)

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

func GetIP(base IP, n int) (net.IP, error) {
	baseip := base.IP.To4()
	if baseip == nil {
		return nil, errors.New("Invalid Interface address!")
	}
	base32 := binary.BigEndian.Uint32(baseip)
	base32 += uint32(n) + 1
	t := make([]byte, 4)
	binary.BigEndian.PutUint32(t[:], base32)
	newip := net.IP(t[:])
	if !base.Net.Contains(newip) {
		return nil, errors.New("Out of Addresses!")
	}
	return newip, nil
}
