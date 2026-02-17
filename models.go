package main

import (
	"net"
	"time"
)

type DomainResult struct {
	Domain    string    `json:"domain"`
	Category  int       `json:"category"`
	Action    string    `json:"action"`
	TTL       int       `json:"ttl"`
	Timestamp time.Time `json:"timestamp"`

	Blocked  bool   `json:"blocked"`
	RealIP   net.IP `json:"-"`
	RealIPv6 net.IP `json:"-"`
}
