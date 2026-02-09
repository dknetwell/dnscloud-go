package main

import (
    "context"
    "fmt"
    "net"
    "time"
    
    "github.com/miekg/dns"
)

// DNSServer - DNS сервер
type DNSServer struct {
    address string
    engine  *CheckEngine
    udpSrv  *dns.Server
    tcpSrv  *dns.Server
}

func newDNSServer(address string, engine *CheckEngine) *DNSServer {
    handler := dns.NewServeMux()
    
    udpSrv := &dns.Server{
        Addr:      address,
        Net:       "udp",
        Handler:   handler,
        UDPSize:   65535,
        ReusePort: true,
    }
    
    tcpSrv := &dns.Server{
        Addr:      address,
        Net:       "tcp",
        Handler:   handler,
        ReusePort: true,
    }
    
    server := &DNSServer{
        address: address,
        engine:  engine,
        udpSrv:  udpSrv,
        tcpSrv:  tcpSrv,
    }
    
    handler.HandleFunc(".", server.handleDNSRequest)
    return server
}

func (s *DNSServer) start() error {
    // Запускаем UDP и TCP серверы
    errChan := make(chan error, 2)
    
    go func() {
        logInfo("Starting DNS UDP server", "address", s.address)
        if err := s.udpSrv.ListenAndServe(); err != nil {
            errChan <- fmt.Errorf("UDP server failed: %w", err)
        }
    }()
    
    go func() {
        logInfo("Starting DNS TCP server", "address", s.address)
        if err := s.tcpSrv.ListenAndServe(); err != nil {
            errChan <- fmt.Errorf("TCP server failed: %w", err)
        }
    }()
    
    // Ждем ошибку от любого сервера
    return <-errChan
}

func (s *DNSServer) shutdown(ctx context.Context) {
    if s.udpSrv != nil {
        s.udpSrv.ShutdownContext(ctx)
    }
    if s.tcpSrv != nil {
        s.tcpSrv.ShutdownContext(ctx)
    }
}

func (s *DNSServer) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
    start := time.Now()
    
    m := new(dns.Msg)
    m.SetReply(r)
    m.Compress = true
    m.Authoritative = true
    
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
        "type", dns.TypeToString[q.Qtype],
        "protocol", w.RemoteAddr().Network())
    
    // Проверяем домен
    ctx := context.Background()
    result, err := s.engine.checkDomain(ctx, domain)
    if err != nil {
        logError("Domain check failed", err, "domain", domain)
        m.Rcode = dns.RcodeServerFailure
    } else {
        // Формируем ответ в зависимости от действия
        if result.Action == "allow" {
            // Для разрешенных доменов возвращаем NXDOMAIN
            // (в реальности можно было бы форвардить запрос дальше)
            m.Rcode = dns.RcodeNameError
        } else {
            // Для заблокированных доменов возвращаем sinkhole IP
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
                
            default:
                // Для других типов запросов возвращаем NOERROR без ответа
                m.Rcode = dns.RcodeSuccess
            }
        }
    }
    
    // Отправляем ответ
    if err := w.WriteMsg(m); err != nil {
        logError("Failed to write DNS response", err)
    }
    
    duration := time.Since(start)
    logDebug("DNS response",
        "client", w.RemoteAddr().String(),
        "domain", domain,
        "action", result.Action,
        "duration_ms", duration.Milliseconds())
}
