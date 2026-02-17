package main

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/sync/singleflight"
)

const maxDNSPacketSize = 1232

type DNSServer struct {
	engine       *CheckEngine
	resolver     *net.Resolver
	sf           singleflight.Group
	sinkholeIPv4 net.IP
	sinkholeIPv6 net.IP
}

func NewDNSServer(engine *CheckEngine, cfg *Config) *DNSServer {

	return &DNSServer{
		engine:       engine,
		resolver:     netResolver,
		sinkholeIPv4: net.ParseIP(cfg.DNS.SinkholeIPv4),
		sinkholeIPv6: net.ParseIP(cfg.DNS.SinkholeIPv6),
	}
}

func (s *DNSServer) HandleDNS(w dns.ResponseWriter, r *dns.Msg) {

	if r.Len() > maxDNSPacketSize {
		return
	}

	m := new(dns.Msg)
	m.SetReply(r)

	for _, q := range r.Question {

		domain := strings.TrimSuffix(q.Name, ".")

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		result, err := s.engine.CheckDomain(ctx, domain)
		cancel()

		if err != nil {
			continue
		}

		switch q.Qtype {

		case dns.TypeA:
			ip := result.RealIP
			if result.Blocked {
				ip = s.sinkholeIPv4
			}
			if ip != nil {
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
			ip := result.RealIPv6
			if result.Blocked {
				ip = s.sinkholeIPv6
			}
			if ip != nil {
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

	_ = w.WriteMsg(m)
}
