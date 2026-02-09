package main

import (
    "context"
    "net"
    "time"
    
    "github.com/miekg/dns"
)

// DNSServer - DNS сервер
type DNSServer struct {
    address string
    engine  *CheckEngine
    server  *dns.Server
}

func newDNSServer(address string, engine *CheckEngine) *DNSServer {
    srv := &dns.Server{
        Addr:      address,
        Net:       "udp",
        UDPSize:   65535,
        ReusePort: true,
    }
    
    return &DNSServer{
        address: address,
        engine:  engine,
        server:  srv,
    }
}

func (s *DNSServer) start() error {
    dns.HandleFunc(".", s.handleDNSRequest)
    return s.server.ListenAndServe()
}

func (s *DNSServer) shutdown(ctx context.Context) error {
    return s.server.ShutdownContext(ctx)
}

func (s *DNSServer) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
    start := time.Now()
    
    m := new(dns.Msg)
    m.SetReply(r)
    m.Compress = true
    
    if len(r.Question) == 0 {
        m.Rcode = dns.RcodeFormatError
        w.WriteMsg(m)
        return
    }
    
    q := r.Question[0]
    domain := q.Name
    
    logDebug("DNS request",
        "client", w.RemoteAddr().String(),
        "domain", domain,
        "type", dns.TypeToString[q.Qtype])
    
    // Проверяем домен
    ctx := context.Background()
    result, err := s.engine.checkDomain(ctx, domain)
    if err != nil {
        logError("Domain check failed", err, "domain", domain)
        // В случае ошибки возвращаем SERVFAIL
        m.Rcode = dns.RcodeServerFailure
    } else {
        // Формируем ответ в зависимости от действия
        if result.Action == "allow" {
            // Разрешаем запрос (возвращаем NXDOMAIN для теста или можно форвардить)
            m.Rcode = dns.RcodeNameError // NXDOMAIN
        } else {
            // Блокируем - возвращаем sinkhole IP
            m.Rcode = dns.RcodeSuccess
            
            // Добавляем ответ в зависимости от типа запроса
            switch q.Qtype {
            case dns.TypeA:
                rr := &dns.A{
                    Hdr: dns.RR_Header{
                        Name:   q.Name,
                        Rrtype: dns.TypeA,
                        Class:  dns.ClassINET,
                        Ttl:    uint32(result.TTL),
                    },
                    A: net.ParseIP(result.IP).To4(),
                }
                m.Answer = append(m.Answer, rr)
                
            case dns.TypeAAAA:
                rr := &dns.AAAA{
                    Hdr: dns.RR_Header{
                        Name:   q.Name,
                        Rrtype: dns.TypeAAAA,
                        Class:  dns.ClassINET,
                        Ttl:    uint32(result.TTL),
                    },
                    AAAA: net.ParseIP("::1"),
                }
                m.Answer = append(m.Answer, rr)
            }
        }
    }
    
    // Отправляем ответ
    w.WriteMsg(m)
    
    duration := time.Since(start)
    logDebug("DNS response",
        "client", w.RemoteAddr().String(),
        "domain", domain,
        "action", result.Action,
        "duration_ms", duration.Milliseconds())
}
