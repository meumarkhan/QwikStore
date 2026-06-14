package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"qwikstore/config"
	"qwikstore/server"
)

func main() {
	cfg := config.FromFlags()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("starting qwikstore on %s", cfg.Addr())
	log.Printf("eviction policy: %s | AOF: %v | databases: %d",
		cfg.EvictionPolicy, cfg.AOFEnabled, cfg.Databases)

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	// Graceful shutdown on signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down...")
		srv.Close()
		os.Exit(0)
	}()

	if err := srv.Listen(); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
