package main

import (
	"log"
	"net"
	"os"

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

	cloudEnricher := NewCloudAPIEnricher(cfg)

	engine := NewCheckEngine(
		cfg,
		cache,
		[]Enricher{
			cloudEnricher,
		},
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

	log.Println("DNS Proxy started on :5353")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}

	defer server.Shutdown()
	os.Exit(0)
}
