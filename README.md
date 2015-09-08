# go-dnsmasq
*Version 0.9.2*

go-dnsmasq is a lightweight (1.4 MB) caching DNS forwarder/proxy optimized for running in Docker containers.

### Notable features (all configurable)

* Uses the nameservers configured in /etc/resolv.conf to forward queries to (can be overriden)
* Manages /etc/resolv.conf to make itself the default nameserver on the host
* Parses the entries of a hostsfile and monitors the file for changes
* Supports caching of answers
* Replicates the `search domain` feature not supported in musl-libc based distros (e.g. Alpine Linux)
* Allows configuration of stubzones to use a different nameserver for certain domain(s)
* Round-robin of address records
* Sending stats to Graphite server
* All options are also configurable through environmental variables

### Commandline options

```sh
NAME:
   go-dnsmasq - Lightweight caching DNS proxy for Docker containers

USAGE:
   go-dnsmasq [global options] command [command options] [arguments...]

COMMANDS:
   help, h	Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --listen, -l "127.0.0.1:53"	listen address: ‘host[:port]‘ [$DNSMASQ_LISTEN]
   --default-resolver, -d	make go-dnsmasq the default name server (updates /etc/resolv.conf) [$DNSMASQ_DEFAULT]
   --nameservers, -n 		comma-separated list of name servers: ‘host[:port]‘ [$DNSMASQ_SERVERS]
   --stubzones, -z 		domains to resolve using a specific nameserver: ‘domain[,domain]/host[:port]‘ [$DNSMASQ_STUB]
   --hostsfile, -f 		full path to hostsfile (e.g. ‘/etc/hosts‘) [$DNSMASQ_HOSTSFILE]
   --hostsfile-poll, -p "0"	how frequently to poll hostsfile (in seconds, ‘0‘ to disable) [$DNSMASQ_POLL]
   --search-domain, -s 		specify search domain (takes precedence over /etc/resolv.conf) [$DNSMASQ_SEARCH]
   --append-domain, -a		enable suffixing single-label queries with search domain [$DNSMASQ_APPEND]
   --rcache, -r "0"		capacity of the response cache (‘0‘ to disable caching) [$DNSMASQ_RCACHE]
   --rcache-ttl "60"		TTL of entries in the response cache [$DNSMASQ_RCACHE_TTL]
   --no-rec			disable recursion [$DNSMASQ_NOREC]
   --round-robin		enable round robin of A/AAAA replies [$DNSMASQ_RR]
   --systemd			bind to socket(s) activated by systemd (ignores --listen) [$DNSMASQ_SYSTEMD]
   --verbose			enable verbose logging [$DNSMASQ_VERBOSE]
   --help, -h			show help
   --version, -v		print the version
```

### Environment Variables

See above (the names inside the brackets).

### Usage

#### Run from the commandline

Download the binary for your OS from the [releases page](https://github.com/janeczku/go-dnsmasq/releases/latest).    

go-dnsmasq is available in two versions. The minimal binary (`go-dnsmasq-min`) has a lower memory footprint but doesn't include caching, stats reporting and systemd support.

   sudo ./go-dnsmasq [options]

#### Run as a Docker container

```sh
docker run -d -e DNSMASQ_LISTEN=0.0.0.0 -p 53:53/udp -p 53:53 \
   janeczku/go-dnsmasq
```

You can configure go-dnsmasq by passing the corresponding environmental variables with docker run `--env` flag.
