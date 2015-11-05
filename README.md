# go-dnsmasq
*Version 0.9.8*

go-dnsmasq is a light weight (1.2 MB) DNS caching server/forwarder with minimal filesystem and runtime overhead. It is designed to serve global DNS records by forwarding queries to upstream nameservers as well as local hostname records from a hostsfile.

### Application examples:

- as local DNS cache for Docker containers
- as nameserver providing local and global DNS records to clients in a private networks 
- as DNS proxy providing `search` domain path capability to `musl-libc` based clients (e.g. Alpine Linux)

### Features

* Parses upstream nameservers from resolv.conf
* Configures itself as local DNS cache in resolv.conf
* Serves static hostname records from a hostsfile
* Caching of answers
* Replicates the `search` domain suffixing not supported by `musl-libc` based Linux distributions.
* Stubzones (use a different nameserver for specific domains)
* Round-robin of address records
* Sending stats to Graphite server
* Configuration through CLI and environment variables

### Resolver logic

DNS queries are processed according to the logic used by the GNU C resolver library:
* The first nameserver (as listed in resolv.conf or configured by `--nameservers`) is considered the primary server. Additional servers are queried only when the primary server times out or returns an error code.
* Multiple `search` paths are tried in the order they are configured. 
* Single-label queries (e.g.: "redis-service") will always be qualified with `search` list elements
* For multi-label queries (ndots >= 1) the name will be tried first as an absolute name before any `search` list elements are appended to it.

### Commandline options

```sh
NAME:
   go-dnsmasq - Lightweight caching DNS proxy for Docker containers

USAGE:
   go-dnsmasq [global options] command [command options] [arguments...]

VERSION:
   0.9.8

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --listen, -l "127.0.0.1:53"   listen address: ‘host[:port]‘ [$DNSMASQ_LISTEN]
   --default-resolver, -d  make go-dnsmasq the local primary nameserver (updates /etc/resolv.conf) [$DNSMASQ_DEFAULT]
   --nameservers, -n       comma-separated list of name servers: ‘host[:port]‘ [$DNSMASQ_SERVERS]
   --stubzones, -z      domains to resolve using a specific nameserver: ‘fqdn[,fqdn]/host[:port]‘ [$DNSMASQ_STUB]
   --hostsfile, -f      full path to hostsfile (e.g. ‘/etc/hosts‘) [$DNSMASQ_HOSTSFILE]
   --hostsfile-poll, -p "0"   how frequently to poll hostsfile (in seconds, ‘0‘ to disable) [$DNSMASQ_POLL]
   --search-domains, -s    specify SEARCH domains taking precedence over /etc/resolv.conf: ‘fqdn[,fqdn]‘ [$DNSMASQ_SEARCH]
   --append-search-domains, -a   enable suffixing single-label queries with SEARCH domains [$DNSMASQ_APPEND]
   --rcache, -r "0"     capacity of the response cache (‘0‘ to disable caching) [$DNSMASQ_RCACHE]
   --rcache-ttl "60"    TTL of entries in the response cache [$DNSMASQ_RCACHE_TTL]
   --no-rec       disable recursion [$DNSMASQ_NOREC]
   --round-robin     enable round robin of A/AAAA replies [$DNSMASQ_RR]
   --systemd         bind to socket(s) activated by systemd (ignores --listen) [$DNSMASQ_SYSTEMD]
   --verbose         enable verbose logging [$DNSMASQ_VERBOSE]
   --syslog       enable syslog logging [$DNSMASQ_SYSLOG]
   --multithreading     enable multithreading (num physical CPU cores) [$DNSMASQ_MULTITHREADING]
   --help, -h        show help
   --version, -v     print the version
```

### Environment Variables

See above (the names inside the brackets).

### Usage

#### Run from the commandline

Download the binary for your OS from the [releases page](https://github.com/janeczku/go-dnsmasq/releases/latest).    

go-dnsmasq is available in two versions. The minimal version (`go-dnsmasq-min`) has a lower memory footprint but doesn't have caching, stats reporting and systemd support.

```sh
   sudo ./go-dnsmasq [options]
```

#### Run as a Docker container

```sh
docker run -d -e DNSMASQ_LISTEN=0.0.0.0 -p 53:53/udp -p 53:53 \
   janeczku/go-dnsmasq
```

You can configure go-dnsmasq by passing the corresponding environmental variables with docker run `--env` flag.
