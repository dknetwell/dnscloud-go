package security

import (
    "context"
    "strings"
    "time"
    
    "github.com/coredns/coredns/plugin"
    "github.com/coredns/coredns/request"
    "github.com/miekg/dns"
    
    "github.com/proxy/dnscloud-go/checker"
    "github.com/proxy/dnscloud-go/config"
    "github.com/proxy/dnscloud-go/logger"
)

type Security struct {
    Next    plugin.Handler
    checker *checker.Engine
    metrics *Metrics
}

func New(next plugin.Handler) *Security {
    // Создаем движок проверки
    engine := checker.NewEngine()
    
    return &Security{
        Next:    next,
        checker: engine,
        metrics: NewMetrics(),
    }
}

func (s *Security) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
    state := request.Request{W: w, Req: r}
    
    // Извлекаем домен
    domain := state.Name()
    domain = strings.TrimSuffix(domain, ".")
    
    // Логируем запрос
    logger.Debug("DNS query received",
        "domain", domain,
        "client", state.IP(),
        "type", dns.TypeToString[state.QType()])
    
    s.metrics.IncRequests()
    start := time.Now()
    
    // Проверяем домен
    result, err := s.checker.Check(ctx, domain)
    if err != nil {
        logger.Error("Domain check failed",
            "domain", domain,
            "error", err)
        
        s.metrics.IncErrors()
        // Fallback к следующему плагину
        return plugin.NextOrFailure(s.Name(), s.Next, ctx, w, r)
    }
    
    // Логируем результат
    logger.Info("Domain check completed",
        "domain", domain,
        "action", result.Action,
        "category", result.Category,
        "source", result.Source,
        "ttl", result.TTL,
        "duration_ms", time.Since(start).Milliseconds())
    
    s.metrics.ObserveDuration(time.Since(start))
    
    // Создаем DNS ответ
    m := s.createDNSResponse(r, result, state.QType())
    
    // Отправляем ответ
    w.WriteMsg(m)
    
    // Обновляем метрики
    if result.Action == "block" {
        s.metrics.IncBlocked(result.Category)
    } else {
        s.metrics.IncAllowed()
    }
    
    return dns.RcodeSuccess, nil
}

func (s *Security) createDNSResponse(query *dns.Msg, result *checker.DomainResult, qtype uint16) *dns.Msg {
    m := new(dns.Msg)
    m.SetReply(query)
    m.Authoritative = true
    m.RecursionAvailable = true
    
    ttl := result.TTL
    
    switch qtype {
    case dns.TypeA:
        // Для A запросов
        if result.IP != "" {
            m.Answer = []dns.RR{
                &dns.A{
                    Hdr: dns.RR_Header{
                        Name:   query.Question[0].Name,
                        Rrtype: dns.TypeA,
                        Class:  dns.ClassINET,
                        Ttl:    ttl,
                    },
                    A: result.IP,
                },
            }
            m.Rcode = dns.RcodeSuccess
        } else {
            // Нет IP - возвращаем NXDOMAIN
            m.Rcode = dns.RcodeNameError
        }
        
    case dns.TypeAAAA:
        // Для AAAA запросов
        if result.Action == "block" {
            // Возвращаем IPv6 sinkhole
            m.Answer = []dns.RR{
                &dns.AAAA{
                    Hdr: dns.RR_Header{
                        Name:   query.Question[0].Name,
                        Rrtype: dns.TypeAAAA,
                        Class:  dns.ClassINET,
                        Ttl:    ttl,
                    },
                    AAAA: config.Get().Sinkholes.IPv6,
                },
            }
            m.Rcode = dns.RcodeSuccess
        } else {
            // Для разрешенных - пустой ответ
            m.Rcode = dns.RcodeSuccess
        }
        
    default:
        // Для других типов запросов
        m.Rcode = dns.RcodeNameError
    }
    
    return m
}

func (s *Security) Name() string { return "security" }
