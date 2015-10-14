// Copyright (c) 2015 Jan Broer. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package main // import "github.com/janeczku/go-dnsmasq"

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/janeczku/go-dnsmasq/dns"

	"github.com/janeczku/go-dnsmasq/hostsfile"
	"github.com/janeczku/go-dnsmasq/resolvconf"
	"github.com/janeczku/go-dnsmasq/server"
)

// var Version string
const Version = "0.9.3"

var (
	nameservers   = []string{}
	searchDomains = []string{}
	stubzones     = ""
	hostPort      = ""
	listen        = ""
)

var exitErr error

func main() {
	app := cli.NewApp()
	app.Name = "go-dnsmasq"
	app.Usage = "Lightweight caching DNS proxy for Docker containers"
	app.Version = Version
	app.Author, app.Email = "", ""
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "listen, l",
			Value:  "127.0.0.1:53",
			Usage:  "listen address: ‘host[:port]‘",
			EnvVar: "DNSMASQ_LISTEN",
		},
		cli.BoolFlag{
			Name:   "default-resolver, d",
			Usage:  "make go-dnsmasq the default name server (updates /etc/resolv.conf)",
			EnvVar: "DNSMASQ_DEFAULT",
		},
		cli.StringFlag{
			Name:   "nameservers, n",
			Value:  "",
			Usage:  "comma-separated list of name servers: ‘host[:port]‘",
			EnvVar: "DNSMASQ_SERVERS",
		},
		cli.StringFlag{
			Name:   "stubzones, z",
			Value:  "",
			Usage:  "domains to resolve using a specific nameserver: ‘fqdn[,fqdn]/host[:port]‘",
			EnvVar: "DNSMASQ_STUB",
		},
		cli.StringFlag{
			Name:   "hostsfile, f",
			Value:  "",
			Usage:  "full path to hostsfile (e.g. ‘/etc/hosts‘)",
			EnvVar: "DNSMASQ_HOSTSFILE",
		},
		cli.IntFlag{
			Name:   "hostsfile-poll, p",
			Value:  0,
			Usage:  "how frequently to poll hostsfile (in seconds, ‘0‘ to disable)",
			EnvVar: "DNSMASQ_POLL",
		},
		cli.StringFlag{
			Name:   "search-domains, s",
			Value:  "",
			Usage:  "Specify SEARCH domains taking precedence over /etc/resolv.conf: ‘fqdn[,fqdn]‘",
			EnvVar: "DNSMASQ_SEARCH",
		},
		cli.BoolFlag{
			Name:   "append-search-domains, a",
			Usage:  "enable suffixing single-label queries with SEARCH domain",
			EnvVar: "DNSMASQ_APPEND",
		},
		cli.BoolFlag{
			Name:   "no-rec",
			Usage:  "disable recursion",
			EnvVar: "DNSMASQ_NOREC",
		},
		cli.BoolFlag{
			Name:   "round-robin",
			Usage:  "enable round robin of A/AAAA replies",
			EnvVar: "DNSMASQ_RR",
		},
		cli.BoolFlag{
			Name:   "systemd",
			Usage:  "bind to socket(s) activated by systemd (ignores --listen)",
			EnvVar: "DNSMASQ_SYSTEMD",
		},
		cli.BoolFlag{
			Name:   "verbose",
			Usage:  "enable verbose logging",
			EnvVar: "DNSMASQ_VERBOSE",
		},
	}
	app.Action = func(c *cli.Context) {
		exitReason := make(chan error)
		go func() {
			c := make(chan os.Signal, 1)
			signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
			sig := <-c
			log.Println("go-dnsmasq: exit requested by signal:", sig)
			exitReason <- nil
		}()

		if ns := c.String("nameservers"); ns != "" {
			for _, hostPort := range strings.Split(ns, ",") {
				if !strings.Contains(hostPort, ":") {
					hostPort += ":53"
				}

				if err := validateHostPort(hostPort); err != nil {
					log.Fatalf("go-dnsmasq: nameserver is invalid: %s", err)
				}

				nameservers = append(nameservers, hostPort)
			}
		}

		if sd := c.String("search-domains"); sd != "" {
			for _, domain := range strings.Split(sd, ",") {

				if dns.CountLabel(domain) < 2 {
					log.Fatalf("go-dnsmasq: SEARCH domain is not a FQDN: %s", domain)
				}
				domain = dns.Fqdn(strings.ToLower(domain))
				searchDomains = append(searchDomains, domain)
			}
		}

		if listen = c.String("listen"); !strings.Contains(listen, ":") {
			listen += ":53"
		}

		if err := validateHostPort(listen); err != nil {
			log.Fatalf("go-dnsmasq: listen address is invalid: %s", err)
		}

		config := &server.Config{
			DnsAddr:         listen,
			DefaultResolver: c.Bool("default-resolver"),
			Nameservers:     nameservers,
			Systemd:         c.Bool("systemd"),
			SearchDomains:   searchDomains,
			AppendDomain:    c.Bool("append-search-domains"),
			Hostsfile:       c.String("hostsfile"),
			PollInterval:    c.Int("hostsfile-poll"),
			RoundRobin:      c.Bool("round-robin"),
			NoRec:           c.Bool("no-rec"),
			ReadTimeout:     0,
			Verbose:         c.Bool("verbose"),
		}

		if err := server.SetDefaults(config); err != nil {
			if !config.NoRec && len(config.Nameservers) == 0 {
				log.Fatalf("go-dnsmasq: found no nameservers in resolv.conf and --nameservers flag not supplied: %s", err)
			} else if config.AppendDomain && len(config.SearchDomains) == 0 {
				log.Fatalf("go-dnsmasq: found no SEARCH domains in resolv.conf and --search-domains flag not supplied: %s", err)
			} else {
				log.Printf("go-dnsmasq: error parsing resolv.conf: %s", err)
			}
		}

		if stubzones = c.String("stubzones"); stubzones != "" {
			stubmap := make(map[string][]string)
			segments := strings.Split(stubzones, "/")
			if len(segments) != 2 || len(segments[0]) == 0 || len(segments[1]) == 0 {
				log.Fatalf("go-dnsmasq: stubzones argument is invalid")
			}

			hostPort = segments[1]
			if !strings.Contains(hostPort, ":") {
				hostPort += ":53"
			}

			if err := validateHostPort(hostPort); err != nil {
				log.Fatalf("go-dnsmasq: stubzones server address invalid: %s", err)
			}

			for _, sdomain := range strings.Split(segments[0], ",") {
				if dns.CountLabel(sdomain) < 2 {
					log.Fatalf("go-dnsmasq: stubzones domain is not a FQDN: %s", sdomain)
				}
				sdomain = dns.Fqdn(sdomain)
				stubmap[sdomain] = append(stubmap[sdomain], hostPort)
			}

			config.Stub = &stubmap
		}

		log.Printf("starting go-dnsmasq %s ...", Version)

		hf, err := hosts.NewHostsfile(config.Hostsfile, &hosts.Config{
			Poll:    config.PollInterval,
			Verbose: config.Verbose,
		})
		if err != nil {
			log.Fatalf("go-dnsmasq: error loading hostsfile: %s", err)
		}

		s := server.New(hf, config, Version)

		defer s.Stop()

		if config.DefaultResolver {
			address, _, _ := net.SplitHostPort(config.DnsAddr)
			err := resolvconf.StoreAddress(address)
			if err != nil {
				log.Printf("go-dnsmasq: failed to register as default resolver: %s", err)
			}
			defer resolvconf.Clean()
		}

		go func() {
			if err := s.Run(); err != nil {
				exitReason <- err
			}
		}()

		exitErr = <-exitReason
		if exitErr != nil {
			log.Fatalf("go-dnsmasq: %s", err)
		}
	}

	app.Run(os.Args)
}

func validateHostPort(hostPort string) error {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip == nil {
		return fmt.Errorf("bad IP address: %s", host)
	}

	if p, _ := strconv.Atoi(port); p < 1 || p > 65535 {
		return fmt.Errorf("bad port number %s", port)
	}
	return nil
}
