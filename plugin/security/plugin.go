package security

import (
    "context"
    "strings"
    "time"

    "github.com/coredns/coredns/plugin"
    "github.com/coredns/coredns/request"
    "github.com/miekg/dns"

    "checker"
    "config"
    "logger"
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
   
