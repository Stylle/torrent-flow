package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"
)

func main() {
	guardian := flag.Bool("guardian", false, "Run independent recovery guardian")
	guardianHealth := flag.Bool("guardian-health", false, "Check guardian heartbeat")
	flag.Parse()
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = "data"
	}
	s, err := openStore(dir)
	if err != nil {
		log.Fatal(err)
	}
	defer s.db.Close()
	if *guardianHealth {
		if time.Now().Unix()-heartbeat(s, "guardian") > 20 {
			os.Exit(1)
		}
		return
	}
	e := newEngine(s)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *guardian {
		log.Print("Torrent Flow recovery guardian started")
		e.guardian(ctx)
		return
	}
	if err = initAdmin(s, dir); err != nil {
		log.Fatal(err)
	}
	sv := &Server{e: e, s: s, attempts: map[string][]time.Time{}}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: sv.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan struct{})
	go func() { defer close(done); e.run(ctx) }()
	go func() {
		<-ctx.Done()
		closeCtx, c := context.WithTimeout(context.Background(), 40*time.Second)
		defer c()
		server.Shutdown(closeCtx)
	}()
	log.Printf("Torrent Flow %s listening on %s", Version, addr)
	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Print(err)
		cancel()
	}
	<-done
}
