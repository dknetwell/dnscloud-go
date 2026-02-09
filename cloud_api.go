package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// CloudAPIClient - клиент Cloud API
type CloudAPIClient struct {
    config *CloudAPIConfig  // Исправлено: указатель
    client *http.Client
}

func newCloudAPIClient(config *CloudAPIConfig) *CloudAPIClient {  // Исправлено: принимает указатель
    return &CloudAPIClient{
        config: config,
        client: &http.Client{
            Timeout: config.Timeout,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
        },
    }
}
