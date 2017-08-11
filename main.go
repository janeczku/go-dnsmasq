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
	"time"

	log "github.com/sirupsen/logrus"
	logrus_syslog "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/codegangsta/cli"
	"github.com/miekg/dns"

	"github.com/janeczku/go-dnsmasq/hostsfile"
	"github.com/janeczku/go-dnsmasq/resolvconf"
	"github.com/janeczku/go-dnsmasq/server"
	"github.com/janeczku/go-dnsmasq/stats"
)

// set at build time
var Version = "dev"

var (
	nameservers   = []string{}
	searchDomains = []string{}
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
	app.Usage = "Lightweight caching DNS server and forwarder\n   Website: http://github.com/janeczku/go-dnsmasq"
	app.UsageText = "go-dnsmasq [global options]"
	app.Version = Version
	app.Author, app.Email = "", ""
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "listen, l",
			Value:  "127.0.0.1:53",
			Usage:  "Listen on this `address` <host[:port]>",
			EnvVar: "DNSMASQ_LISTEN",
		},
		cli.BoolFlag{
			Name:   "default-resolver, d",
			Usage:  "Update /etc/resolv.conf with the address of go-dnsmasq as nameserver",
			EnvVar: "DNSMASQ_DEFAULT",
		},
		cli.StringFlag{
			Name:   "nameservers, n",
			Value:  "",
			Usage:  "Comma delimited list of `nameservers` <host[:port][,host[:port]]> (supersedes resolv.conf)",
			EnvVar: "DNSMASQ_SERVERS",
		},
		cli.StringSliceFlag{
			Name:   "stubzones, z",
			Usage:  "Use different nameservers for given domains <domain[,domain]/host[:port][,host[:port]]>",
			EnvVar: "DNSMASQ_STUB",
		},
		cli.StringFlag{
			Name:   "hostsfile, f",
			Value:  "",
			Usage:  "Path to a hosts `file` (e.g. /etc/hosts)",
			EnvVar: "DNSMASQ_HOSTSFILE",
		},
		cli.IntFlag{
			Name:   "hostsfile-poll, p",
			Value:  0,
			Usage:  "How frequently to poll hosts file (`seconds`, '0' to disable)",
			EnvVar: "DNSMASQ_POLL",
		},
		cli.StringFlag{
			Name:   "search-domains, s",
			Value:  "",
			Usage:  "List of search domains <domain[,domain]> (supersedes resolv.conf)",
			EnvVar: "DNSMASQ_SEARCH_DOMAINS,DNSMASQ_SEARCH,", // deprecated DNSMASQ_SEARCH
		},
		cli.BoolFlag{ // deprecated
			Name:   "append-search-domains, a",
			Usage:  "Resolve queries using search domains",
			EnvVar: "DNSMASQ_APPEND",
			Hidden: true,
		},
		cli.BoolFlag{
			Name:   "enable-search, search",
			Usage:  "Qualify names with search domains to resolve queries",
			EnvVar: "DNSMASQ_ENABLE_SEARCH",
		},
		cli.IntFlag{
			Name:   "rcache, r",
			Value:  0,
			Usage:  "Response cache `capacity` ('0' disables caching)",
			EnvVar: "DNSMASQ_RCACHE",
		},
		cli.IntFlag{
			Name:   "rcache-ttl",
			Value:  60,
			Usage:  "TTL in `seconds` for response cache entries",
			EnvVar: "DNSMASQ_RCACHE_TTL",
		},
		cli.BoolFlag{
			Name:   "no-rec",
			Usage:  "Disable recursion",
			EnvVar: "DNSMASQ_NOREC",
		},
		cli.IntFlag{
			Name:   "fwd-ndots",
			Value:  0,
			Usage:  "Number of `dots` a name must have before the query is forwarded",
			EnvVar: "DNSMASQ_FWD_NDOTS",
		},
		cli.IntFlag{
			Name:   "ndots",
			Value:  1,
			Usage:  "Number of `dots` a name must have before doing an initial absolute query (supersedes resolv.conf)",
			EnvVar: "DNSMASQ_NDOTS",
		},
		cli.BoolFlag{
			Name:   "round-robin",
			Usage:  "Enable round robin of A/AAAA records",
			EnvVar: "DNSMASQ_RR",
		},
		cli.BoolFlag{
			Name:   "systemd",
			Usage:  "Bind to socket activated by Systemd (supersedes '--listen')",
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
			Usage:  "Enable multithreading (experimental)",
			EnvVar: "DNSMASQ_MULTITHREADING",
		},
	}
	app.Action = func(c *cli.Context) error {
		exitReason := make(chan error)
		go func() {
			c := make(chan os.Signal, 1)
			signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
			sig := <-c
			log.Infoln("Application exit requested by signal:", sig)
			exitReason <- nil
		}()

		var enableSearch bool
		if c.IsSet("append-search-domains") {
			log.Info("The flag '--append-search-domains' is deprecated. Please use '--enable-search' or '-search' instead.")
			enableSearch = c.Bool("append-search-domains")
		} else {
			enableSearch = c.Bool("enable-search")
		}

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
			log.SetFormatter(&log.TextFormatter{})
		}

		if ns := c.String("nameservers"); ns != "" {
			for _, hostPort := range strings.Split(ns, ",") {
				hostPort = strings.TrimSpace(hostPort)
				if strings.HasSuffix(hostPort, "]") {
					hostPort += ":53"
				} else if !strings.Contains(hostPort, ":") {
					hostPort += ":53"
				}
				if err := validateHostPort(hostPort); err != nil {
					log.Fatalf("Nameserver is invalid: %s", err)
				}

				nameservers = append(nameservers, hostPort)
			}
		}

		if sd := c.String("search-domains"); sd != "" {
			for _, domain := range strings.Split(sd, ",") {
				if dns.CountLabel(domain) < 2 {
					log.Fatalf("Search domain must have at least one dot in name: %s", domain)
				}
				domain = strings.TrimSpace(domain)
				domain = dns.Fqdn(strings.ToLower(domain))
				searchDomains = append(searchDomains, domain)
			}
		}

		listen = c.String("listen")
		if strings.HasSuffix(listen, "]") {
			listen += ":53"
		} else if !strings.Contains(listen, ":") {
			listen += ":53"
		}

		if err := validateHostPort(listen); err != nil {
			log.Fatalf("Listen address is invalid: %s", err)
		}

		config := &server.Config{
			DnsAddr:         listen,
			DefaultResolver: c.Bool("default-resolver"),
			Nameservers:     nameservers,
			Systemd:         c.Bool("systemd"),
			SearchDomains:   searchDomains,
			EnableSearch:    enableSearch,
			Hostsfile:       c.String("hostsfile"),
			PollInterval:    c.Int("hostsfile-poll"),
			RoundRobin:      c.Bool("round-robin"),
			NoRec:           c.Bool("no-rec"),
			FwdNdots:        c.Int("fwd-ndots"),
			Ndots:           c.Int("ndots"),
			ReadTimeout:     2 * time.Second,
			RCache:          c.Int("rcache"),
			RCacheTtl:       c.Int("rcache-ttl"),
			Verbose:         c.Bool("verbose"),
		}

		resolvconf.Clean()
		if err := server.ResolvConf(config, c); err != nil {
			if !os.IsNotExist(err) {
				log.Warnf("Error parsing resolv.conf: %s", err.Error())
			}
		}

		if err := server.CheckConfig(config); err != nil {
			log.Fatal(err.Error())
		}

		if stubzones := c.StringSlice("stubzones"); len(stubzones) > 0 {
			stubmap := make(map[string][]string)
			for _, stubzone := range stubzones {
				segments := strings.Split(stubzone, "/")
				if len(segments) != 2 || len(segments[0]) == 0 || len(segments[1]) == 0 {
					log.Fatalf("Invalid value for --stubzones")
				}

				hosts := strings.Split(segments[1], ",")
				for _, hostPort := range hosts {
					hostPort = strings.TrimSpace(hostPort)
					if strings.HasSuffix(hostPort, "]") {
						hostPort += ":53"
					} else if !strings.Contains(hostPort, ":") {
						hostPort += ":53"
					}

					if err := validateHostPort(hostPort); err != nil {
						log.Fatalf("Stubzone server address is invalid: %s", err)
					}

					for _, sdomain := range strings.Split(segments[0], ",") {
						if dns.CountLabel(sdomain) < 1 {
							log.Fatalf("Stubzone domain is not a fully-qualified domain name: %s", sdomain)
						}
						sdomain = strings.TrimSpace(sdomain)
						sdomain = dns.Fqdn(sdomain)
						stubmap[sdomain] = append(stubmap[sdomain], hostPort)
					}
				}
			}
			config.Stub = &stubmap
		}

		log.Infof("Starting go-dnsmasq server %s", Version)
		log.Infof("Nameservers: %v", config.Nameservers)
		if config.EnableSearch {
			log.Infof("Search domains: %v", config.SearchDomains)
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

			defer func() {
				log.Info("Restoring /etc/resolv.conf")
				resolvconf.Clean()
			}()
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

		return nil
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
