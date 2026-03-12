package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type DNSServer struct {
	engine *CheckEngine
	cfg    *Config
	client *dns.Client
}

func NewDNSServer(engine *CheckEngine, cfg *Config) *DNSServer {
	return &DNSServer{
		engine: engine,
		cfg:    cfg,
		client: &dns.Client{
			Timeout: 2 * time.Second,
		},
	}
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
	if len(r.Question) == 0 {
		return
	}

	start := time.Now()
	requestsTotal.Inc()

	q := r.Question[0]
	domain := strings.TrimSuffix(q.Name, ".")
	qtype := dns.TypeToString[q.Qtype]

	clientIP := ""
	if addr := w.RemoteAddr(); addr != nil {
		host, _, err := net.SplitHostPort(addr.String())
		if err == nil {
			clientIP = host
		} else {
			clientIP = addr.String()
		}
	}

	// FIX: пропускаем запросы к корневой зоне "." (NS ., SOA . и т.п.)
	// TrimSuffix(".", ".") == "" — пустой домен нельзя обогащать через CloudAPI,
	// он вызывает "api status: error" и засоряет метрики/логи.
	if domain == "" {
		resp, err := s.forwardToUpstream(r)
		if err != nil {
			m := new(dns.Msg)
			m.SetRcode(r, dns.RcodeServerFailure)
			_ = w.WriteMsg(m)
			return
		}
		_ = w.WriteMsg(resp)
		return
	}

	result, _ := s.engine.CheckDomain(domain)

	latencyMs := float64(time.Since(start).Microseconds()) / 1000.0
	blocked := result != nil && result.Blocked

	blockedLabel := "false"
	if blocked {
		blockedLabel = "true"
		requestsBlocked.Inc()
	}
	requestDuration.WithLabelValues(blockedLabel).Observe(latencyMs)

	category := 0
	action := "allow"
	source := "engine"
	ttl := 300
	if result != nil {
		category = result.Category
		action = result.Action
		source = result.Source
		ttl = result.TTL
	}

	LogDNSRequest(domain, clientIP, qtype, action, source, latencyMs, ttl, blocked, category)

	if blocked {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		s.writeSinkhole(m, q, result)
		_ = w.WriteMsg(m)
		return
	}

	resp, err := s.forwardToUpstream(r)
	if err != nil {
		LogError("dns", "upstream failed", err)
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeServerFailure)
		_ = w.WriteMsg(m)
		return
	}

	resp.Authoritative = false
	_ = w.WriteMsg(resp)
}

// forwardToUpstream пробует все upstream серверы с общим контекстным таймаутом.
// FIX: добавлен context.WithTimeout чтобы суммарное время ожидания по всем
// upstream не превышало (2s * кол-во upstream). Это предотвращает накопление
// задержки когда несколько upstream недоступны одновременно.
func (s *DNSServer) forwardToUpstream(r *dns.Msg) (*dns.Msg, error) {
	// Общий таймаут = 2s на upstream × количество upstream (обычно 2 → 4s max)
	totalTimeout := time.Duration(len(s.cfg.DNS.Upstream)) * 2 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), totalTimeout)
	defer cancel()

	for _, upstream := range s.cfg.DNS.Upstream {
		// Проверяем не истёк ли общий таймаут перед следующей попыткой
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("all upstreams failed: context deadline")
		default:
		}

		resp, _, err := s.client.Exchange(r, upstream)
		if err == nil && resp != nil && resp.Rcode == dns.RcodeSuccess {
			return resp, nil
		}
	}
	return nil, fmt.Errorf("all upstreams failed")
}

func (s *DNSServer) writeSinkhole(m *dns.Msg, q dns.Question, result *DomainResult) {
	ipv4 := s.cfg.DNS.SinkholeIPv4
	ipv6 := s.cfg.DNS.SinkholeIPv6

	if result != nil {
		if result.SinkholeIPv4 != "" {
			ipv4 = result.SinkholeIPv4
		}
		if result.SinkholeIPv6 != "" {
			ipv6 = result.SinkholeIPv6
		}
	}

	switch q.Qtype {
	case dns.TypeA:
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP(ipv4),
		})
	case dns.TypeAAAA:
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			AAAA: net.ParseIP(ipv6),
		})
	}
}
