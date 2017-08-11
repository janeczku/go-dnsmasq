// Copyright (c) 2015 Jan Broer. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package server

import (
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/miekg/dns"
)

// Config provides options to the go-dnsmasq resolver
type Config struct {
	// The ip:port go-dnsmasq should be listening on for incoming DNS requests.
	DnsAddr string `json:"dns_addr,omitempty"`
	// bind to port(s) activated by systemd. If set to true, this overrides DnsAddr.
	Systemd bool `json:"systemd,omitempty"`
	// Rewrite host's network config making go-dnsmasq the default resolver
	DefaultResolver bool `json:"default_resolver,omitempty"`
	// Search domains used to qualify queries
	SearchDomains []string `json:"search_domains,omitempty"`
	// Replicates GNU libc's use of /etc/resolv.conf search domains
	EnableSearch bool `json:"append_domain,omitempty"`
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
	// RCache, capacity of response cache in resource records stored.
	RCache int `json:"rcache,omitempty"`
	// RCacheTtl, how long to cache in seconds.
	RCacheTtl int `json:"rcache_ttl,omitempty"`
	// How many dots a name must have before we allow to forward the query as-is. Defaults to 1.
	FwdNdots int `json:"fwd_ndots,omitempty"`
	// How many dots a name must have before we do an initial absolute query. Defaults to 1.
	Ndots int `json:"ndots,omitempty"`

	Verbose bool `json:"-"`

	// Stub zones support. Map contains domainname -> nameserver:port
	Stub *map[string][]string
}

func ResolvConf(config *Config, ctx *cli.Context) error {
	// Get host resolv config
	resolvConf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return err
	}

	if len(config.Nameservers) == 0 {
		for _, s := range resolvConf.Servers {
			config.Nameservers = append(config.Nameservers, net.JoinHostPort(s, resolvConf.Port))
		}
	}

	if !ctx.IsSet("ndots") && resolvConf.Ndots != 1 {
		log.Debugf("Setting ndots from resolv.conf: %d", resolvConf.Ndots)
		config.Ndots = resolvConf.Ndots
	}

	if config.EnableSearch && len(config.SearchDomains) == 0 {
		for _, s := range resolvConf.Search {
			s = dns.Fqdn(strings.ToLower(s))
			config.SearchDomains = append(config.SearchDomains, s)
		}
	}

	return nil
}

func CheckConfig(config *Config) error {
	if config.DnsAddr == "" {
		return fmt.Errorf("'listen' cannot be empty")
	}
	if !config.NoRec && len(config.Nameservers) == 0 {
		return fmt.Errorf("Recursion is enabled but no nameservers are configured")
	}
	if config.EnableSearch && len(config.SearchDomains) == 0 {
		config.EnableSearch = false
		log.Warnf("No search domains configured, disabling search.")
	}
	if config.RCache < 0 {
		return fmt.Errorf("'rcache' must be equal or greater than 0")
	}
	if config.RCacheTtl <= 0 {
		return fmt.Errorf("'rcache-ttl' must be greater than 0")
	}
	if config.Ndots <= 0 {
		return fmt.Errorf("'ndots' must be greater than 0")
	}
	if config.FwdNdots < 0 {
		return fmt.Errorf("'fwd-ndots' must be equal or greater than 0")
	}

	// Set defaults
	config.Ttl = 360
	config.HostsTtl = 10

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
