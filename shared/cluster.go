package shared

import (
	"sync"
)

type Cluster struct {
    Servers []ServerConfig
    Leader  ServerConfig
    mu      sync.Mutex
    Index   int 
}

func (c *Cluster) GetNextServer() ServerConfig {
    c.mu.Lock()
    defer c.mu.Unlock()
    server := c.Servers[c.Index]
    c.Index = (c.Index + 1) % len(c.Servers)
    return server
}

func (c *Cluster) GetNextServerIndex() int {
    c.mu.Lock()
    defer c.mu.Unlock()
    index := c.Index
    c.Index = (c.Index + 1) % len(c.Servers)
    return index
}
