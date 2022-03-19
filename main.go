package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/miekg/dns"
)

const defaultTTL = 3600

func main() {
	port := flag.Int("port", 1053, "DNS server listening port")
	ip4 := flag.String("ip4", "127.0.0.1", "Default A record response IP")
	ip6 := flag.String("ip6", "::1", "Default AAAA record response IP")
	upstream := flag.String("upstream", "8.8.8.8", "Upstream DNS resolver")
	forward := flag.String("forward", "connectivitycheck.gstatic.com, pool.ntp.org", "A record names to forward to upstream DNS (separate multiple values by comma)")
	logDate := flag.Bool("log-date", true, "include date/time in log output")
	flag.Parse()

	if !*logDate {
		log.SetFlags(0)
	}

	if *upstream == "" {
		log.Fatal("-upstream cannot be blank")
	}
	respA := net.ParseIP(*ip4)
	respAAAA := net.ParseIP(*ip6)
	if len(respA) == 0 {
		log.Fatalf("invalid IPv4 %q", *ip4)
	}
	if len(respAAAA) == 0 {
		log.Fatalf("invalid IPv6 %q", *ip4)
	}

	forwardNames := strings.Split(*forward, ",")
	for i, n := range forwardNames {
		n = strings.TrimSpace(n)
		l := len(n)
		if l != 0 && n[l-1] != '.' {
			// DNS queries for domains end in '.'.
			n = n + "."
		}
		forwardNames[i] = n
	}

	h := dnsHandler{
		respA:    respA,
		respAAAA: respAAAA,
		upstream: *upstream,
		forward:  forwardNames,
	}

	dsrv := dns.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Net:     "udp",
		Handler: &h,
	}
	go func() {
		if err := dsrv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	dsrv.Shutdown()
}

type dnsHandler struct {
	respA    net.IP
	respAAAA net.IP
	upstream string
	forward  []string
}

func (d *dnsHandler) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	var q dns.Question
	if len(req.Question) > 0 {
		q = req.Question[0]
	}

	var (
		respIP  net.IP
		respTTL uint32
	)
	defer func() {
		var ip string
		if len(respIP) == 0 {
			ip = "(Refused)"
		} else {
			ip = respIP.String()
		}
		log.Printf("DNS: % -4s %s > %s (%d)", dns.Type(q.Qtype), q.Name, ip, respTTL)
	}()

	var resp dns.Msg
	resp.Authoritative = true
	resp.SetReply(req)

	shouldForward := func(name string) bool {
		for _, n := range d.forward {
			if n == name {
				return true
			}
		}
		return false
	}

	switch q.Qtype {

	case dns.TypeAAAA:
		if shouldForward(q.Name) {
			in, err := dns.Exchange(req, fmt.Sprintf("%s:53", d.upstream))
			if err != nil {
				log.Printf("Error requesting from upstream DNS: %s", err)
			} else {
				if a, ok := in.Answer[0].(*dns.AAAA); ok {
					respIP = a.AAAA
					respTTL = a.Hdr.Ttl
				} else {
					log.Printf("Upstream response not an AAAA record; got %T", in.Answer[0])
				}
			}
		}
		if len(respIP) == 0 {
			respIP = d.respAAAA
		}
		if respTTL <= 0 {
			respTTL = defaultTTL
		}

		resp.Answer = append(resp.Answer, &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    respTTL,
			},
			AAAA: respIP.To16(),
		})

	case dns.TypeA:
		if shouldForward(q.Name) {
			in, err := dns.Exchange(req, fmt.Sprintf("%s:53", d.upstream))
			if err != nil {
				log.Printf("Error requesting from upstream DNS: %s", err)
			} else {
				if a, ok := in.Answer[0].(*dns.A); ok {
					respIP = a.A
					respTTL = a.Hdr.Ttl
				} else {
					log.Printf("Upstream response not an A record; got %T", in.Answer[0])
				}
			}
		}
		if len(respIP) == 0 {
			respIP = d.respA
		}
		if respTTL <= 0 {
			respTTL = defaultTTL
		}

		resp.Answer = append(resp.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    respTTL,
			},
			A: respIP.To4(),
		})

	default:
		resp.SetRcode(req, dns.RcodeRefused)
	}

	if err := w.WriteMsg(&resp); err != nil {
		log.Printf("error responding to %q: %s", req, err)
	}
}
