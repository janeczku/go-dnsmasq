// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Copyright (c) 2015 Jan Broer. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package server

import (
	"net"
	"os"
	"strings"
	"time"

	"github.com/janeczku/go-dnsmasq/dns"
)

// Config provides options to the go-dnsmasq resolver
type Config struct {
	// The ip:port go-dnsmasq should be listening on for incoming DNS requests.
	DnsAddr string `json:"dns_addr,omitempty"`
	// Rewrite host's network config making go-dnsmasq the default resolver
	DefaultResolver bool `json:"default_resolver,omitempty"`
	// Domain to append to query names that are not FQDN
	// Replicates the SEARCH keyword in /etc/resolv.conf
	SearchDomains []string `json:"search_domains,omitempty"`
	// Replicates the SEARCH keyword in /etc/resolv.conf
	AppendDomain bool `json:"append_domain,omitempty"`
	// Path to the hostfile
	Hostsfile string `json:"hostfile,omitempty"`
	// Hostfile Polling
	PollInterval int `json:"poll_interval,omitempty"`
	// Round robin A/AAAA replies. Default is true.
	RoundRobin bool `json:"round_robin,omitempty"`
	// List of ip:port, seperated by commas of recursive nameservers to forward queries to.
	Nameservers []string `json:"nameservers,omitempty"`
	// Never provide a recursive service.
	NoRec       bool          `json:"no_rec,omitempty"`
	ReadTimeout time.Duration `json:"read_timeout,omitempty"`
	// Default TTL, in seconds. Defaults to 360.
	Ttl uint32 `json:"ttl,omitempty"`
	// Default TTL for Hostfile records, in seconds. Defaults to 30.
	HostsTtl uint32 `json:"hostfile_ttl,omitempty"`
	// How many labels a name should have before we allow forwarding. Default to 2.
	Ndots int `json:"ndot,omitempty"`

	Verbose bool `json:"-"`

	// Stub zones support. Map contains domainname -> nameserver:port
	Stub *map[string][]string
}

func SetDefaults(config *Config) error {
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 2 * time.Second
	}
	if config.DnsAddr == "" {
		config.DnsAddr = "127.0.0.1:53"
	}
	if config.Ttl == 0 {
		config.Ttl = 360
	}
	if config.HostsTtl == 0 {
		config.HostsTtl = 10
	}
	if config.Ndots <= 0 {
		config.Ndots = 2
	}

	if len(config.Nameservers) == 0 {
		c, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if !os.IsNotExist(err) {
			if err != nil {
				return err
			}
			for _, s := range c.Servers {
				config.Nameservers = append(config.Nameservers, net.JoinHostPort(s, c.Port))
			}
		}
	}

	// For now we only get the first SEARCH domain found
	if config.AppendDomain && len(config.SearchDomains) == 0 {
		c, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if !os.IsNotExist(err) {
			if err != nil {
				return err
			}
			for _, s := range c.Search {
				s = dns.Fqdn(strings.ToLower(s))
				config.SearchDomains = append(config.SearchDomains, s)
			}
		}
	}

	stubmap := make(map[string][]string)
	config.Stub = &stubmap
	return nil
}

func appendDomain(s1, s2 string) string {
	if len(s2) > 0 && strings.HasPrefix(s2, ".") {
		strings.TrimLeft(s2, ".")
	}
	return dns.Fqdn(s1) + dns.Fqdn(s2)
}
