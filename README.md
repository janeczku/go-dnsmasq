# go-dnsmasq
*Version 1.0.2*

go-dnsmasq is a light weight (1.2 MB) DNS caching server/forwarder with minimal filesystem and runtime overhead.

### Application examples:

- Caching DNS server/forwarder in a local network
- Container/Host DNS cache
- DNS proxy providing DNS `search` capabilities to `musl-libc` based clients, particularly Alpine Linux

### Features

* Automatically set upstream `nameservers` and `search` domains from resolv.conf
* Insert itself into the host's /etc/resolv.conf on start
* Serve static A/AAAA records from a hostsfile
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
| --nameservers, -n              | Comma separated list of nameservers `host[:port]`                             | -             | $DNSMASQ_SERVERS     |
| --stubzones, -z                | Use different nameservers for specific domains `domain[,domain]/host[:port]`  | -             | $DNSMASQ_STUB        |
| --hostsfile, -f                | Path to a hostsfile (e.g. ‘/etc/hosts‘)                                       | -             | $DNSMASQ_HOSTSFILE   |
| --hostsfile-poll, -p           | How frequently to poll hostsfile for changes (seconds, ‘0‘ to disable)        | 0             | $DNSMASQ_POLL        |
| --search-domains, -s           | Specify search domains (overrides /etc/resolv.conf) `domain[,domain]`         | -             | $DNSMASQ_SEARCH      |
| --append-search-domains, -a    | Resolve queries using search domains                                          | False         | $DNSMASQ_APPEND      |
| --rcache, -r                   | Capacity of the response cache (‘0‘ to disable cache)                         | 0             | $DNSMASQ_RCACHE      |
| --rcache-ttl                   | TTL for entries in the response cache                                         | 60            | $DNSMASQ_RCACHE_TTL  |
| --no-rec                       | Disable recursion                                                             | False         | $DNSMASQ_NOREC       |
| --round-robin                  | Enable round robin of A/AAAA records                                          | False         | $DNSMASQ_RR          |
| --systemd                      | Bind to socket(s) activated by Systemd (ignores --listen)                     | False         | $DNSMASQ_SYSTEMD     |
| --verbose                      | Enable verbose logging                                                        | False         | $DNSMASQ_VERBOSE     |
| --syslog                       | Enable syslog logging                                                         | False         | $DNSMASQ_SYSLOG      |
| --multithreading               | Enable multithreading                                                         | False         |                      |
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

#### Run from the commandline

Download the binary for your OS from the [releases page](https://github.com/janeczku/go-dnsmasq/releases/latest).    

go-dnsmasq is available in two versions. The minimal version (`go-dnsmasq-min`) has a lower memory footprint but doesn't have caching, stats reporting and systemd support.

```sh
   sudo ./go-dnsmasq [options]
```

#### Run as a Docker container
[![ImageLayers Size](https://img.shields.io/imagelayers/image-size/janeczku/go-dnsmasq/latest.svg)]() [![Docker Pulls](https://img.shields.io/docker/pulls/janeczku/go-dnsmasq.svg)]()

Docker Hub trusted builds [available](https://hub.docker.com/r/janeczku/go-dnsmasq/).

```sh
docker run -d -p 53:53/udp -p 53:53 janeczku/go-dnsmasq:latest
```

You can configure the container by passing the corresponding environmental variables with docker run's `--env` flag.

#### Serving A/AAAA records from a hosts file
The `--hostsfile` parameter expects a standard plain text [hosts file](https://en.wikipedia.org/wiki/Hosts_(file)) with the only difference being that a wildcard `*` in the left-most label of hostnames is allowed. Wildcard entries will match any subdomain that is not explicitely defined.
For example, given a hosts file with the following content:

```
192.168.0.1 db1.db.local
192.168.0.2 *.db.local
```

Queries for `db2.db.local` would be answered with an A record pointing to 192.168.0.2, while queries for `db1.db.local` would yield an A record pointing to 192.168.0.1.
