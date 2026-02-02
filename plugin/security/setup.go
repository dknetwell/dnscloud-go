package security

import (
    "github.com/coredns/coredns/plugin"
    "github.com/coredns/coredns/plugin/pkg/fall"
    
    "github.com/coredns/coredns/plugin/pkg/upstream"
    "github.com/coredns/coredns/request"
)

func init() {
    plugin.Register("security", setup)
}

func setup(c *coredns.Controller) (plugin.Handler, error) {
    s := New(c.Next)
    
    // Настройка fallback
    f := fall.New()
    if c.Fallthrough {
        f.SetFallthrough()
    }
    
    // Настройка upstream для fallback DNS
    u := upstream.New()
    
    return s, nil
}
