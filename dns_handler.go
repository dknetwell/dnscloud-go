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
    m.Rcode = dns.RcodeSuccess // По умолчанию успех

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

    // Проверяем домен параллельно с получением реального IP
    ctx, cancel := context.WithTimeout(context.Background(), getConfig().Timeouts.Total)
    defer cancel()

    // Каналы для параллельного выполнения
    checkResultChan := make(chan *DomainResult, 1)
    realIPChan := make(chan string, 1)
    errorChan := make(chan error, 2)

    // 1. Проверка категории домена (параллельно)
    go func() {
        result, err := s.engine.checkDomain(ctx, domain)
        if err != nil {
            errorChan <- err
            return
        }
        checkResultChan <- result
    }()

    // 2. Получение реального IP из публичного DNS (параллельно)
    go func() {
        ip, err := s.resolveRealIP(ctx, domain)
        if err != nil {
            errorChan <- err
            return
        }
        realIPChan <- ip
    }()

    // Ждем результаты
    var checkResult *DomainResult
    var action string
    var responseIP string

    select {
    case result := <-checkResultChan:
        checkResult = result
        action = result.Action
        
        // Если домен разрешен - получаем реальный IP
        if result.Action == "allow" {
            select {
            case ip := <-realIPChan:
                responseIP = ip
            case <-time.After(50 * time.Millisecond):
                // Если не успели получить IP - используем fallback
                logWarn("Real IP resolution timeout", "domain", domain)
                responseIP = "8.8.8.8" // Fallback IP
            }
        } else {
            // Если домен заблокирован - используем sinkhole IP
            responseIP = result.IP
        }
        
    case ip := <-realIPChan:
        // Если проверка категории не успела - разрешаем домен
        action = "allow"
        responseIP = ip
        
    case <-ctx.Done():
        // SLA превышен - разрешаем домен с fallback IP
        logWarn("SLA timeout in DNS handler", "domain", domain)
        action = "allow"
        responseIP = "8.8.8.8"
    }

    // Если checkResult nil (не успели получить), создаем fallback
    if checkResult == nil {
        checkResult = &DomainResult{
            Category: 0,
            TTL:      getTTLByCategory(0),
        }
    }

    // Формируем ответ в зависимости от типа запроса
    switch q.Qtype {
    case dns.TypeA:
        rr := &dns.A{
            Hdr: dns.RR_Header{
                Name:   q.Name,
                Rrtype: dns.TypeA,
                Class:  dns.ClassINET,
                Ttl:    uint32(checkResult.TTL),
            },
            A: net.ParseIP(responseIP).To4(),
        }
        m.Answer = append(m.Answer, rr)

    case dns.TypeAAAA:
        rr := &dns.AAAA{
            Hdr: dns.RR_Header{
                Name:   q.Name,
                Rrtype: dns.TypeAAAA,
                Class:  dns.ClassINET,
                Ttl:    uint32(checkResult.TTL),
            },
            AAAA: net.ParseIP("::1"), // IPv6 sinkhole
        }
        m.Answer = append(m.Answer, rr)

    default:
        // Для других типов запросов возвращаем NOERROR без ответа
        m.Rcode = dns.RcodeSuccess
    }

    // Отправляем ответ
    if err := w.WriteMsg(m); err != nil {
        logError("Failed to write DNS response", err)
    }

    duration := time.Since(start)
    logDebug("DNS response",
        "client", w.RemoteAddr().String(),
        "domain", domain,
        "action", action,
        "ip", responseIP,
        "duration_ms", duration.Milliseconds())
}

// resolveRealIP - получение реального IP из публичного DNS
func (s *DNSServer) resolveRealIP(ctx context.Context, domain string) (string, error) {
    // Очищаем точку в конце
    cleanDomain := domain
    if len(cleanDomain) > 0 && cleanDomain[len(cleanDomain)-1] == '.' {
        cleanDomain = cleanDomain[:len(cleanDomain)-1]
    }

    // Используем системные резолверы с таймаутом
    resolver := &net.Resolver{
        PreferGo: true,
        Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
            d := net.Dialer{
                Timeout: 50 * time.Millisecond,
            }
            // Используем публичные DNS серверы
            return d.DialContext(ctx, network, "8.8.8.8:53")
        },
    }

    // Разрешаем домен
    addrs, err := resolver.LookupHost(ctx, cleanDomain)
    if err != nil {
        return "", err
    }

    if len(addrs) == 0 {
        return "", fmt.Errorf("no IP found for domain %s", cleanDomain)
    }

    // Возвращаем первый IPv4 адрес
    for _, addr := range addrs {
        if ip := net.ParseIP(addr); ip != nil && ip.To4() != nil {
            return addr, nil
        }
    }

    return "", fmt.Errorf("no IPv4 address found for domain %s", cleanDomain)
}
