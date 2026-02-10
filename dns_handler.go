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
    m.Rcode = dns.RcodeSuccess

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

    // КОНТЕКСТ ДЛЯ КЛИЕНТА - SLA 100ms
    clientCtx, clientCancel := context.WithTimeout(context.Background(), getConfig().Timeouts.Total)
    defer clientCancel()

    // Контекст для Cloud API (дольше, 2 секунды)
    apiCtx, apiCancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer apiCancel()

    // Каналы для параллельного выполнения
    cacheResultChan := make(chan *DomainResult, 1)
    realIPChan := make(chan string, 1)
    apiResultChan := make(chan *DomainResult, 1)

    // 1. Проверка кеша (самый быстрый путь)
    go func() {
        if cached := s.engine.cache.get(domain); cached != nil {
            cached.Source = "cache"
            cacheResultChan <- cached
        }
    }()

    // 2. Получение реального IP из публичного DNS (параллельно)
    go func() {
        ip, err := s.resolveRealIP(clientCtx, domain)
        if err != nil {
            logWarn("Failed to resolve real IP", "domain", domain, "error", err)
            realIPChan <- "8.8.8.8" // Fallback
        } else {
            realIPChan <- ip
        }
    }()

    // 3. Запуск проверки Cloud API в фоне (для обогащения кеша)
    // Этот запрос продолжается даже после ответа клиенту
    go func() {
        // Используем отдельный контекст с таймаутом 2 секунды
        result, err := s.engine.checkDomainForCache(apiCtx, domain)
        if err != nil {
            logDebug("Background API check failed", "domain", domain, "error", err)
        } else {
            apiResultChan <- result
            logDebug("Background API check completed", "domain", domain, "category", result.Category)
        }
    }()

    // Ждем результаты в рамках SLA клиента (100ms)
    var responseIP string
    var ttl uint32 = 300
    var action = "allow" // По умолчанию разрешаем

    select {
    case cached := <-cacheResultChan:
        // Есть в кеше - используем результат
        logDebug("Cache hit", "domain", domain)
        if cached.Action == "block" {
            action = "block"
            responseIP = cached.IP
        } else {
            // Ждем реальный IP
            select {
            case ip := <-realIPChan:
                responseIP = ip
            case <-clientCtx.Done():
                responseIP = "8.8.8.8"
            }
        }
        ttl = uint32(cached.TTL)

    case ip := <-realIPChan:
        // Получили реальный IP, кеша нет
        logDebug("Cache miss, using real IP", "domain", domain)
        responseIP = ip
        // Разрешаем домен (ждем API для кеша в фоне)

    case <-clientCtx.Done():
        // SLA превышен - отдаем fallback
        logWarn("Client SLA timeout", "domain", domain, "timeout", getConfig().Timeouts.Total)
        responseIP = "8.8.8.8"
        action = "allow"
    }

    // Формируем ответ
    switch q.Qtype {
    case dns.TypeA:
        rr := &dns.A{
            Hdr: dns.RR_Header{
                Name:   q.Name,
                Rrtype: dns.TypeA,
                Class:  dns.ClassINET,
                Ttl:    ttl,
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
                Ttl:    ttl,
            },
            AAAA: net.ParseIP("::1"),
        }
        m.Answer = append(m.Answer, rr)
    }

    // Отправляем ответ клиенту (в рамках SLA)
    if err := w.WriteMsg(m); err != nil {
        logError("Failed to write DNS response", err)
    }

    duration := time.Since(start)
    logDebug("DNS response sent",
        "client", w.RemoteAddr().String(),
        "domain", domain,
        "action", action,
        "ip", responseIP,
        "duration_ms", duration.Milliseconds())

    // В фоне продолжаем ждать API результат для кеша (если еще не было)
    go func() {
        select {
        case apiResult := <-apiResultChan:
            // Обогащаем кеш результатом API
            logDebug("Enriching cache with API result", "domain", domain, "category", apiResult.Category)
        case <-time.After(2 * time.Second):
            // Таймаут фоновой проверки
        }
    }()
}

func (s *DNSServer) resolveRealIP(ctx context.Context, domain string) (string, error) {
    cleanDomain := strings.TrimSuffix(domain, ".")

    resolver := &net.Resolver{
        PreferGo: true,
        Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
            d := net.Dialer{
                Timeout: 50 * time.Millisecond,
            }
            // Используем несколько публичных DNS для надежности
            dnsServers := []string{"8.8.8.8:53", "1.1.1.1:53", "9.9.9.9:53"}
            for _, server := range dnsServers {
                conn, err := d.DialContext(ctx, network, server)
                if err == nil {
                    return conn, nil
                }
            }
            return nil, fmt.Errorf("all DNS servers failed")
        },
    }

    // Таймаут на разрешение DNS
    resolveCtx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
    defer cancel()

    addrs, err := resolver.LookupHost(resolveCtx, cleanDomain)
    if err != nil {
        return "", err
    }

    for _, addr := range addrs {
        if ip := net.ParseIP(addr); ip != nil && ip.To4() != nil {
            return addr, nil
        }
    }

    return "", fmt.Errorf("no IPv4 address found")
}
