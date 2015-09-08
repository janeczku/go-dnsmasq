// Copyright (c) 2015 Jan Broer
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package hosts

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

type hostlist []*hostname

type hostname struct {
	domain string
	ip     net.IP
	ipv6   bool
}

// newHostlist creates a hostlist by parsing a file
func newHostlist(data []byte) *hostlist {
	hostlist := hostlist{}
	for _, v := range strings.Split(string(data), "\n") {
		for _, hostname := range parseLine(v) {
			err := hostlist.add(hostname)
			if err != nil {
				log.Printf("go-dnsmasq: warning: %s", err)
			}
		}
	}
	return &hostlist
}

func (h *hostlist) add(hostnamev *hostname) error {
	hostname := newHostname(hostnamev.domain, hostnamev.ip, hostnamev.ipv6)
	for _, found := range *h {
		if found.domain == hostname.domain && found.ip.Equal(hostname.ip) {
			return fmt.Errorf("Duplicate hostname entry for %s -> %s",
				hostname.domain, hostname.ip)
		}
	}
	*h = append(*h, hostname)
	return nil
}

// newHostname creates a new Hostname struct
func newHostname(domain string, ip net.IP, ipv6 bool) (host *hostname) {
	domain = strings.ToLower(domain)
	host = &hostname{domain, ip, ipv6}
	return
}

// ParseLine parses an individual line in a hostfile, which may contain one
// (un)commented ip and one or more hostnames. For example
//
//	127.0.0.1 localhost mysite1 mysite2
func parseLine(line string) hostlist {
	var hostnames hostlist

	if len(line) == 0 {
		return hostnames
	}

	// Parse leading # for disabled lines
	if line[0:1] == "#" {
		return hostnames
	}

	// Parse other #s for actual comments
	line = strings.Split(line, "#")[0]

	// Replace tabs and multispaces with single spaces throughout
	line = strings.Replace(line, "\t", " ", -1)
	for strings.Contains(line, "  ") {
		line = strings.Replace(line, "  ", " ", -1)
	}

	line = strings.TrimSpace(line)

	// Break line into words
	words := strings.Split(line, " ")
	for idx, word := range words {
		words[idx] = strings.TrimSpace(word)
	}

	// Separate the first bit (the ip) from the other bits (the domains)
	address := words[0]
	domains := words[1:]

	if strings.Contains(address, "%") {
		return hostnames
	}

	ip := net.ParseIP(address)

	var isIPv6 bool

	switch {
	case !ip.IsGlobalUnicast():
		return hostnames
	case ip.To4() != nil:
		isIPv6 = false
	case ip.To16() != nil:
		isIPv6 = true
	default:
		log.Printf("go-dnsmasq: notice: invalid IP address found in hostsfile: %s", address)
		return hostnames
	}

	for _, v := range domains {
		hostname := newHostname(v, ip, isIPv6)
		hostnames = append(hostnames, hostname)
	}

	return hostnames
}

// hostsFileMetadata returns metadata about the hosts file.
func hostsFileMetadata(path string) (time.Time, int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, 0, err
	}

	return fi.ModTime(), fi.Size(), nil
}
