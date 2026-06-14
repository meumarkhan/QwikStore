package config

import (
	"flag"
	"fmt"
)

type AOFSync string

const (
	AOFSyncAlways   AOFSync = "always"
	AOFSyncEverySec AOFSync = "everysec"
	AOFSyncNo       AOFSync = "no"
)

type EvictionPolicy string

const (
	EvictNoEviction     EvictionPolicy = "noeviction"
	EvictAllKeysLRU     EvictionPolicy = "allkeys-lru"
	EvictVolatileLRU    EvictionPolicy = "volatile-lru"
	EvictAllKeysLFU     EvictionPolicy = "allkeys-lfu"
	EvictVolatileLFU    EvictionPolicy = "volatile-lfu"
	EvictAllKeysRandom  EvictionPolicy = "allkeys-random"
	EvictVolatileRandom EvictionPolicy = "volatile-random"
	EvictVolatileTTL    EvictionPolicy = "volatile-ttl"
)

type Config struct {
	Host           string
	Port           int
	MaxMemory      int64
	EvictionPolicy EvictionPolicy
	AOFEnabled     bool
	AOFFilename    string
	AOFSync        AOFSync
	Databases      int
	MaxClients     int
}

func Default() *Config {
	return &Config{
		Host:           "127.0.0.1",
		Port:           6379,
		MaxMemory:      0,
		EvictionPolicy: EvictNoEviction,
		AOFEnabled:     false,
		AOFFilename:    "appendonly.aof",
		AOFSync:        AOFSyncEverySec,
		Databases:      16,
		MaxClients:     10000,
	}
}

func FromFlags() *Config {
	cfg := Default()
	flag.StringVar(&cfg.Host, "host", cfg.Host, "Bind address")
	flag.IntVar(&cfg.Port, "port", cfg.Port, "Port to listen on")
	flag.Int64Var(&cfg.MaxMemory, "maxmemory", cfg.MaxMemory, "Max memory in bytes (0 = unlimited)")
	policy := flag.String("maxmemory-policy", string(cfg.EvictionPolicy), "Eviction policy")
	flag.BoolVar(&cfg.AOFEnabled, "appendonly", cfg.AOFEnabled, "Enable AOF persistence")
	flag.StringVar(&cfg.AOFFilename, "appendfilename", cfg.AOFFilename, "AOF filename")
	aofSync := flag.String("appendfsync", string(cfg.AOFSync), "AOF fsync strategy: always|everysec|no")
	flag.IntVar(&cfg.MaxClients, "maxclients", cfg.MaxClients, "Max simultaneous clients")
	flag.Parse()

	cfg.EvictionPolicy = EvictionPolicy(*policy)
	cfg.AOFSync = AOFSync(*aofSync)
	return cfg
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
