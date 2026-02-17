package main

import "context"

type Enricher interface {
	Name() string
	Enrich(ctx context.Context, domain string, result *DomainResult) error
}
