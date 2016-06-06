# go-dnsmasq
[![Latest Version](https://img.shields.io/github/release/janeczku/go-dnsmasq.svg?maxAge=60)][release]
[![Github All Releases](https://img.shields.io/github/downloads/janeczku/go-dnsmasq/total.svg?maxAge=86400)]()
[![Docker Pulls](https://img.shields.io/docker/pulls/janeczku/go-dnsmasq.svg?maxAge=86400)][hub]
[![License](https://img.shields.io/github/license/janeczku/go-dnsmasq.svg?maxAge=86400)]()

[release]: https://github.com/janeczku/go-dnsmasq/releases
[hub]: https://hub.docker.com/r/janeczku/go-dnsmasq/

go-dnsmasq is a lightweight (1.2 MB) DNS caching server/forwarder with minimal filesystem and runtime overhead.

### Application examples:

- Caching DNS server/forwarder in a local network
- Container/Host DNS cache
- DNS proxy providing DNS `search` capabilities to `musl-libc` based clients, particularly Alpine Linux

### Features

* Automatically set upstream `nameservers` and `search` domains from resolv.conf
* Insert itself into the host's /etc/resolv.conf on start
* Serve static A/AAAA records from a hosts file
* Provide DNS response caching
* Replicate the `search` domain treatment not supported by `musl-libc` based Linux distributions
* Supports virtually unlimited number of `search` paths and `nameservers` ([related Kubernetes article](https://github.com/kubernetes/kubernetes/tree/master/cluster/addons/dns#known-issues))
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

### Command-line options / environment variables

| Flag                           | Description                                                                   | Default       | Environment vars     |
| ------------------------------ | ----------------------------------------------------------------------------- | ------------- | -------------------- |
| --listen, -l                   | Address to listen on  `host[:port]`                                           | 127.0.0.1:53  | $DNSMASQ_LISTEN      |
| --default-resolver, -d         | Update resolv.conf to make go-dnsmasq the host's nameserver                   | False         | $DNSMASQ_DEFAULT     |
| --nameservers, -n              | Comma delimited list of nameservers `host[:port]`. IPv6 literal address must be enclosed in brackets. (supersedes etc/resolv.conf) | -  | $DNSMASQ_SERVERS     |
| --stubzones, -z                | Use different nameservers for given domains. Can be passed multiple times. `domain[,domain]/host[:port][,host[:port]]`   | -  |$DNSMASQ_STUB        |
| --hostsfile, -f                | Path to a hosts file (e.g. ‘/etc/hosts‘)                                      | -             | $DNSMASQ_HOSTSFILE   |
| --hostsfile-poll, -p           | How frequently to poll hosts file for changes (seconds, ‘0‘ to disable)       | 0             | $DNSMASQ_POLL        |
| --search-domains, -s           | Comma delimited list of search domains `domain[,domain]` (supersedes /etc/resolv.conf) | -             | $DNSMASQ_SEARCH_DOMAINS      |
| --enable-search, -search       | Qualify names with search domains to resolve queries                          | False         | $DNSMASQ_ENABLE_SEARCH      |
| --rcache, -r                   | Capacity of the response cache (‘0‘ disables caching)                         | 0             | $DNSMASQ_RCACHE      |
| --rcache-ttl                   | TTL for entries in the response cache                                         | 60            | $DNSMASQ_RCACHE_TTL  |
| --no-rec                       | Disable forwarding of queries to upstream nameservers                         | False         | $DNSMASQ_NOREC       |
| --fwd-ndots                    | Number of dots a name must have before the query is forwarded                 | 0 | $DNSMASQ_FWD_NDOTS   |
| --ndots                        | Number of dots a name must have before making an initial absolute query (supersedes /etc/resolv.conf) | 1  | $DNSMASQ_NDOTS |
| --round-robin                  | Enable round robin of A/AAAA records                                          | False         | $DNSMASQ_RR          |
| --systemd                      | Bind to socket(s) activated by Systemd (ignores --listen)                     | False         | $DNSMASQ_SYSTEMD     |
| --verbose                      | Enable verbose logging                                                        | False         | $DNSMASQ_VERBOSE     |
| --syslog                       | Enable syslog logging                                                         | False         | $DNSMASQ_SYSLOG      |
| --multithreading               | Enable multithreading (experimental)                                          | False         |                      |
| --help, -h                     | Show help                                                                     |               |                      |
| --version, -v                  | Print the version                                                             |               |                      |

#### Enable Graphite/StatHat metrics

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

#### Run from the command line

Download the binary for your OS from the [releases page](https://github.com/janeczku/go-dnsmasq/releases/latest).    

go-dnsmasq is available in two versions. The minimal version (`go-dnsmasq-min`) has a lower memory footprint but doesn't have caching, stats reporting and systemd support.

```sh
   sudo ./go-dnsmasq [options]
```

#### Run as a Docker container

Docker Hub trusted builds are [available](https://hub.docker.com/r/janeczku/go-dnsmasq/).

```sh
docker run -d -p 53:53/udp -p 53:53 janeczku/go-dnsmasq:latest
```

You can pass go-dnsmasq configuration parameters by setting the corresponding environmental variables with Docker's `-e` flag.

#### Serving A/AAAA records from a hosts file
The `--hostsfile` parameter expects a standard plain text [hosts file](https://en.wikipedia.org/wiki/Hosts_(file)) with the only difference being that a wildcard `*` in the left-most label of hostnames is allowed. Wildcard entries will match any subdomain that is not explicitly defined.
For example, given a hosts file with the following content:

```
192.168.0.1 db1.db.local
192.168.0.2 *.db.local
```

Queries for `db2.db.local` would be answered with an A record pointing to 192.168.0.2, while queries for `db1.db.local` would yield an A record pointing to 192.168.0.1.
