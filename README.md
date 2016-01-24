# go-dnsmasq
*Version 0.9.8*

go-dnsmasq is a light weight (1.2 MB) DNS caching server/forwarder with minimal filesystem and runtime overhead.

### Application examples:

- Caching DNS server/forwarder in a local network
- Container/Host DNS cache
- DNS proxy providing DNS `search` capabilities to `musl-libc` based clients, particularly Alpine Linux

### Features

* Automatically set upstream `nameservers` and `search` domains from resolv.conf
* Automatically set go-dnsmasq as primary nameserver for the host it is running on
* Serve static records from a hostsfile
* Provide DNS answer caching
* Replicate the `search` domain treatment not supported by `musl-libc` based Linux distributions
* Configure stubzones (different nameserver for specific domains)
* Round-robin of DNS records
* Send server metrics to Graphite and StatHat
* Configuration through both command line flags and environment variables

### Resolve logic

DNS queries are resolved in the style of the GNU libc resolver:
* The first nameserver (as listed in resolv.conf or configured by `--nameservers`) is always queried first, additional servers are considered fallbacks
* Multiple `search` domains are tried in the order they are configured. 
* Single-label queries (e.g.: "redis-service") are always qualified with the `search` domains
* Multi-label queries (ndots >= 1) are first tried as absolute names before qualifying them with the `search` domains

### Command-line options

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

See command-line options.

##### Enable Graphite/StatHat metrics using environment variables

EnvVar: **GRAPHITE_SERVER**  
Default: ` `  
Set to the `host:port` of the Graphite server

EnvVar: **GRAPHITE_PREFIX**  
Default: `go-dnsmasq`  
Set a custom prefix for Graphite metrics

EnvVar: **STATHAT_USER**  
Default: ` `  
Set to your StatHat account email address

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
