// serviceauthd is a standalone cluster authentication server.
// It runs independently of Aegis — services register on startup, receive a
// shared cluster secret, and then communicate directly with HMAC tickets.
//
//	serviceauthd --port=7381 --db=./serviceauth.db
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aegis/internal/serviceauth"
	_ "modernc.org/sqlite"
)

func main() {
	port := flag.String("port", "7381", "listen port")
	dbPath := flag.String("db", "./serviceauth.db", "sqlite database path")
	allowedCIDR := flag.String("allowed-cidr", "10.0.0.0/8,172.16.0.0/12,192.168.0.0/16", "comma-separated CIDR ranges for cluster membership")
	flag.Parse()

	// ① Database.
	db, err := sql.Open("sqlite", *dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := runMigrations(db); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	// ② Dependencies — all pure stdlib implementations.
	svc, err := serviceauth.NewService(serviceauth.Dependencies{
		Repo:        serviceauth.NewRepository(db),
		Secrets:     newFileSecretStore("./cluster_secret.key"),
		NodeChecker: newCIDRChecker(parseCIDRs(*allowedCIDR)),
		LogWriter:   newDBLogWriter(db),
		IDGen:       serviceauth.DefaultIDGen,
		MasterKey:   loadOrGenerateKey("./master.key"),
	})
	if err != nil {
		log.Fatalf("init service: %v", err)
	}

	// ③ HTTP routes.
	mux := http.NewServeMux()
	registerRoutes(mux, svc)

	srv := &http.Server{
		Addr:         ":" + *port,
		Handler:      withLogging(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ④ Graceful shutdown.
	go func() {
		log.Printf("serviceauthd listening on :%s", *port)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

// withLogging wraps a handler with basic request logging.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}
