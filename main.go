package main

import (
	"log"
	"net"

	"github.com/miekg/dns"
)

var netResolver *net.Resolver

func main() {

	cfg, err := LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	initMetrics()

	cache := NewCache(cfg)

	cloud := NewCloudAPIEnricher(cfg)

	engine := NewCheckEngine(
		cfg,
		cache,
		[]Enricher{cloud},
	)

	netResolver = &net.Resolver{
		PreferGo: true,
	}

	dnsServer := NewDNSServer(engine, cfg)

	dns.HandleFunc(".", dnsServer.HandleDNS)

	server := &dns.Server{
		Addr: ":5353",
		Net:  "udp",
	}

	log.Println("DNS proxy started on :5353")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
