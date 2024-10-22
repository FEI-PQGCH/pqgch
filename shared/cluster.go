package shared

import (
	"sync"
)

// ServerConfig represents the configuration for a server
// type ServerConfig struct {
//     Address string `json:"address"`
//     Port    int    `json:"port"`
// }

// Cluster manages a list of servers and implements round-robin logic
type Cluster struct {
    Servers []ServerConfig
    Leader  ServerConfig
    mu      sync.Mutex
    Index   int // Exported to be accessible from other packages
}

// GetNextServer returns the next server in the round-robin sequence
func (c *Cluster) GetNextServer() ServerConfig {
    c.mu.Lock()
    defer c.mu.Unlock()
    server := c.Servers[c.Index]
    c.Index = (c.Index + 1) % len(c.Servers)
    return server
}

// GetNextServerIndex returns the index of the next server
func (c *Cluster) GetNextServerIndex() int {
    c.mu.Lock()
    defer c.mu.Unlock()
    index := c.Index
    c.Index = (c.Index + 1) % len(c.Servers)
    return index
}
