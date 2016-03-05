// Copyright (c) 2015 Jan Broer. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package main // import "github.com/janeczku/go-dnsmasq"

import (
	"fmt"
	"log/syslog"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	log "github.com/Sirupsen/logrus"
	logrus_syslog "github.com/Sirupsen/logrus/hooks/syslog"
	"github.com/codegangsta/cli"
	"github.com/miekg/dns"

	"github.com/janeczku/go-dnsmasq/hostsfile"
	"github.com/janeczku/go-dnsmasq/resolvconf"
	"github.com/janeczku/go-dnsmasq/server"
	"github.com/janeczku/go-dnsmasq/stats"
)

// var Version string
const Version = "1.0.1"

var (
	nameservers   = []string{}
	searchDomains = []string{}
	stubzones     = ""
	hostPort      = ""
	listen        = ""
)

var exitErr error

func init() {
	log.SetOutput(os.Stdout)
}

func main() {
	app := cli.NewApp()
	app.Name = "go-dnsmasq"
	app.Usage = "Lightweight caching DNS server/forwarder"
	app.Version = Version
	app.Author, app.Email = "", ""
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "listen, l",
			Value:  "127.0.0.1:53",
			Usage:  "Address to listen on `host[:port]`",
			EnvVar: "DNSMASQ_LISTEN",
		},
		cli.BoolFlag{
			Name:   "default-resolver, d",
			Usage:  "Update resolv.conf to make go-dnsmasq the host's nameserver",
			EnvVar: "DNSMASQ_DEFAULT",
		},
		cli.StringFlag{
			Name:   "nameservers, n",
			Value:  "",
			Usage:  "Comma separated list of nameservers `host[:port]`",
			EnvVar: "DNSMASQ_SERVERS",
		},
		cli.StringFlag{
			Name:   "stubzones, z",
			Value:  "",
			Usage:  "Use different nameservers for specific domains `domain[,domain]/host[:port]`",
			EnvVar: "DNSMASQ_STUB",
		},
		cli.StringFlag{
			Name:   "hostsfile, f",
			Value:  "",
			Usage:  "Path to a hostsfile (e.g. ‘/etc/hosts‘)",
			EnvVar: "DNSMASQ_HOSTSFILE",
		},
		cli.IntFlag{
			Name:   "hostsfile-poll, p",
			Value:  0,
			Usage:  "How frequently to poll hostsfile for changes (seconds, ‘0‘ to disable)",
			EnvVar: "DNSMASQ_POLL",
		},
		cli.StringFlag{
			Name:   "search-domains, s",
			Value:  "",
			Usage:  "Specify search domains (overrides /etc/resolv.conf) `domain[,domain]`",
			EnvVar: "DNSMASQ_SEARCH",
		},
		cli.BoolFlag{
			Name:   "append-search-domains, a",
			Usage:  "Resolve queries using search domains",
			EnvVar: "DNSMASQ_APPEND",
		},
		cli.IntFlag{
			Name:   "rcache, r",
			Value:  0,
			Usage:  "Capacity of the response cache (‘0‘ to disable cache)",
			EnvVar: "DNSMASQ_RCACHE",
		},
		cli.IntFlag{
			Name:   "rcache-ttl",
			Value:  server.RCacheTtl,
			Usage:  "TTL for entries in the response cache",
			EnvVar: "DNSMASQ_RCACHE_TTL",
		},
		cli.BoolFlag{
			Name:   "no-rec",
			Usage:  "Disable recursion",
			EnvVar: "DNSMASQ_NOREC",
		},
		cli.BoolFlag{
			Name:   "round-robin",
			Usage:  "Enable round robin of A/AAAA records",
			EnvVar: "DNSMASQ_RR",
		},
		cli.BoolFlag{
			Name:   "systemd",
			Usage:  "Bind to socket(s) activated by Systemd (ignores --listen)",
			EnvVar: "DNSMASQ_SYSTEMD",
		},
		cli.BoolFlag{
			Name:   "verbose",
			Usage:  "Enable verbose logging",
			EnvVar: "DNSMASQ_VERBOSE",
		},
		cli.BoolFlag{
			Name:   "syslog",
			Usage:  "Enable syslog logging",
			EnvVar: "DNSMASQ_SYSLOG",
		},
		cli.BoolFlag{
			Name:   "multithreading",
			Usage:  "Enable multithreading",
			EnvVar: "DNSMASQ_MULTITHREADING",
		},
	}
	app.Action = func(c *cli.Context) {
		exitReason := make(chan error)
		go func() {
			c := make(chan os.Signal, 1)
			signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
			sig := <-c
			log.Infoln("Application exit requested by signal:", sig)
			exitReason <- nil
		}()

		if c.Bool("multithreading") {
			runtime.GOMAXPROCS(runtime.NumCPU() + 1)
		}

		if c.Bool("verbose") {
			log.SetLevel(log.DebugLevel)
		}

		if c.Bool("syslog") {
			log.SetFormatter(&log.TextFormatter{DisableTimestamp: true, DisableColors: true})
			hook, err := logrus_syslog.NewSyslogHook("", "", syslog.LOG_DAEMON|syslog.LOG_INFO, "go-dnsmasq")
			if err != nil {
				log.Error("Unable to connect to local syslog daemon")
			} else {
				log.AddHook(hook)
			}
		} else {
			log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})
		}

		if ns := c.String("nameservers"); ns != "" {
			for _, hostPort := range strings.Split(ns, ",") {
				if !strings.Contains(hostPort, ":") {
					hostPort += ":53"
				}

				if err := validateHostPort(hostPort); err != nil {
					log.Fatalf("This nameserver is invalid: %s", err)
				}

				nameservers = append(nameservers, hostPort)
			}
		}

		if sd := c.String("search-domains"); sd != "" {
			for _, domain := range strings.Split(sd, ",") {

				if dns.CountLabel(domain) < 1 {
					log.Fatalf("This search domain is not a FQDN: %s", domain)
				}
				domain = dns.Fqdn(strings.ToLower(domain))
				searchDomains = append(searchDomains, domain)
			}
		}

		if listen = c.String("listen"); !strings.Contains(listen, ":") {
			listen += ":53"
		}

		if err := validateHostPort(listen); err != nil {
			log.Fatalf("The listen address is invalid: %s", err)
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
			RCache:          c.Int("rcache"),
			RCacheTtl:       c.Int("rcache-ttl"),
			Verbose:         c.Bool("verbose"),
		}

		if err := server.SetDefaults(config); err != nil {
			if !config.NoRec && len(config.Nameservers) == 0 {
				log.Fatalf("No nameservers found in resolv.conf and --nameservers option not supplied: %s", err)
			} else if config.AppendDomain && len(config.SearchDomains) == 0 {
				log.Fatalf("No search domains found in resolv.conf and --search-domains option not supplied: %s", err)
			} else {
				log.Warnf("Error parsing resolv.conf: %s", err)
			}
		}

		if stubzones = c.String("stubzones"); stubzones != "" {
			stubmap := make(map[string][]string)
			segments := strings.Split(stubzones, "/")
			if len(segments) != 2 || len(segments[0]) == 0 || len(segments[1]) == 0 {
				log.Fatalf("The --stubzones argument is invalid")
			}

			hostPort = segments[1]
			if !strings.Contains(hostPort, ":") {
				hostPort += ":53"
			}

			if err := validateHostPort(hostPort); err != nil {
				log.Fatalf("This stubzones server address invalid: %s", err)
			}

			for _, sdomain := range strings.Split(segments[0], ",") {
				if dns.CountLabel(sdomain) < 1 {
					log.Fatalf("This stubzones domain is not a FQDN: %s", sdomain)
				}
				sdomain = dns.Fqdn(sdomain)
				stubmap[sdomain] = append(stubmap[sdomain], hostPort)
			}

			config.Stub = &stubmap
		}

		log.Infof("Starting go-dnsmasq server %s ...", Version)
		if config.AppendDomain {
			log.Infof("Search domains in use: %v", config.SearchDomains)
		}

		hf, err := hosts.NewHostsfile(config.Hostsfile, &hosts.Config{
			Poll:    config.PollInterval,
			Verbose: config.Verbose,
		})
		if err != nil {
			log.Fatalf("Error loading hostsfile: %s", err)
		}

		s := server.New(hf, config, Version)

		defer s.Stop()

		stats.Collect()

		if config.DefaultResolver {
			address, _, _ := net.SplitHostPort(config.DnsAddr)
			err := resolvconf.StoreAddress(address)
			if err != nil {
				log.Warnf("Failed to register as default nameserver: %s", err)
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
			log.Fatalf("Server error: %s", err)
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
		return fmt.Errorf("Bad IP address: %s", host)
	}

	if p, _ := strconv.Atoi(port); p < 1 || p > 65535 {
		return fmt.Errorf("Bad port number %s", port)
	}
	return nil
}
