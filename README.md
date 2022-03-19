# virtu

Stop Vizio TV from accessing the internet *and* from starting an open WiFi access point.

### About

"_[Vizio makes nearly as much money from ads and data as it does from TVs](https://www.engadget.com/vizio-q1-earnings-inscape-013937337.html)_".

Vizio televisions can be unplugged from the internet, unfortunately this may trigger the television to stand up an open WiFi access point (`Television.e000`).
What follows is a (convoluted) setup to block the television from the internet, along with preventing it from creating an AP.

### Connectivity Check

Through experimentation, it was found that the television will *not* start an AP if:
 * It can make a HTTPS requests to `connectivitycheck.gstatic.com`
 * It can make NTP requests

The following firewall/iptables rule can be used to block all traffic *except*:
 * TCP traffic on port 443 (HTTPS) to the IP range of `connectivitycheck.gstatic.com`
 * UDP traffic on port 123 (NTP)

The IP range for `connectivitycheck.gstatic.com` (in this case `142.250.0.0/15`) was determined by:
 1. Doing a DNS lookup of `connectivitycheck.gstatic.com`
 2. Then finding the corresponding block in https://www.gstatic.com/ipranges/goog.json

```
Chain Blackhole (1 references)
target     prot opt source               destination
RETURN     udp  --  0.0.0.0/0            0.0.0.0/0            /* Blackhole-10 */ state NEW,RELATED,ESTABLISHED match-set Blackhole src udp dpt:123
RETURN     tcp  --  0.0.0.0/0            142.250.0.0/15       /* Blackhole-20 */ state NEW,RELATED,ESTABLISHED match-set Blackhole src tcp dpt:443
DROP       all  --  0.0.0.0/0            0.0.0.0/0            /* Blackhole-30 */ match-set Blackhole src
RETURN     all  --  0.0.0.0/0            0.0.0.0/0            /* Blackhole-10000 default-action accept */
```

The match-set "Blackhole" was assigned to the television's IP.

### DNS

As an additional step, the television's DNS can be redirected to the DNS resolver found in this repository.
The resolver returns the loopback address to all queries except those in a "forward" list (default is `connectivitycheck.gstatic.com, pool.ntp.org`).

Vizio televisions have been found to make a significant number of DNS requests.

#### Usage

Build:
```shell
go build
```

Run:
```shell
sudo ./virtu -port 53
```

#### dnsmasq

The DNS settings of the television can be set manually.

If using `dnsmasq` as a DHCP server, the following configuration can be used to assign the television a separate DNS resolver (such as `virtu`).

```
# Set hosts tagged with 'blackhole-dns' to use 10.0.1.8 as their DNS server.
dhcp-option=tag:blackhole-dns,option:dns-server,10.0.1.8
```

Then "tag" hosts with `blackhole-dns` to assign them the alternative DNS.
```
# tv
dhcp-host=aa:bb:cc:dd:ee:ff,set:blackhole-dns,10.0.1.110,tv
```
