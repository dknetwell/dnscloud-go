package main

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

var netResolver = &net.Resolver{}

type DNSServer struct {
	engine *CheckEngine
	cfg    *Config
}

func NewDNSServer(engine *CheckEngine, cfg *Config) *DNSServer {
	return &DNSServer{engine: engine, cfg: cfg}
}

func (s *DNSServer) Start() {

	dns.HandleFunc(".", s.handleDNS)

	udpServer := &dns.Server{
		Addr:         s.cfg.DNS.ListenUDP,
		Net:          "udp",
		UDPSize:      s.cfg.DNS.MaxPacketSize,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	}

	tcpServer := &dns.Server{
		Addr:         s.cfg.DNS.ListenTCP,
		Net:          "tcp",
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	}

	go udpServer.ListenAndServe()
	go tcpServer.ListenAndServe()

	LogInfo("dns", "DNS server started")
}

func (s *DNSServer) handleDNS(w dns.ResponseWriter, r *dns.Msg) {

	if r.Len() > s.cfg.DNS.MaxPacketSize {
		return
	}

	if len(r.Question) == 0 {
		return
	}

	requestsTotal.Inc()

	q := r.Question[0]
	domain := strings.TrimSuffix(q.Name, ".")

	result, _ := s.engine.CheckDomain(domain)

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	if result.Blocked {
		s.writeSinkhole(m, q)
	} else {
		s.resolveReal(m, q, result)
	}

	_ = w.WriteMsg(m)
}

func (s *DNSServer) writeSinkhole(m *dns.Msg, q dns.Question) {

	switch q.Qtype {

	case dns.TypeA:
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP(s.cfg.DNS.SinkholeIPv4),
		})

	case dns.TypeAAAA:
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			AAAA: net.ParseIP(s.cfg.DNS.SinkholeIPv6),
		})
	}
}

func (s *DNSServer) resolveReal(m *dns.Msg, q dns.Question, result *DomainResult) {

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	switch q.Qtype {

	case dns.TypeA:
		ips, _ := netResolver.LookupIP(ctx, "ip4", result.Domain)
		for _, ip := range ips {
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    uint32(result.TTL),
				},
				A: ip,
			})
		}

	case dns.TypeAAAA:
		ips, _ := netResolver.LookupIP(ctx, "ip6", result.Domain)
		for _, ip := range ips {
			m.Answer = append(m.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    uint32(result.TTL),
				},
				AAAA: ip,
			})
		}
	}
}
