package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aegis/internal/config"
	"aegis/internal/httpapi"
	"aegis/internal/token"

	"github.com/spf13/cobra"
)

func newServeCommand(cfg *config.Config, svcs *httpapi.Services) *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Aegis HTTP API server",
		Long:  "Starts the Aegis HTTP API server. Defaults to 127.0.0.1 only.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if addr == "" {
				addr = cfg.Server.Addr
			}
			if addr == "" {
				addr = "127.0.0.1:7380"
			}

			// Set up auth middleware
			auth := token.NewAuthMiddleware(cfg.Server.AdminToken, nil)
			apiMiddleware := httpapi.NewMiddleware(auth)

			// Set up routes
			mux := http.NewServeMux()
			httpapi.RegisterRoutes(mux, svcs)

			// Wrap with auth and CORS
			var handler http.Handler = mux
			handler = apiMiddleware.CORS(handler)
			handler = apiMiddleware.Auth(handler)

			srv := &http.Server{
				Addr:         addr,
				Handler:      handler,
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 10 * time.Second,
				IdleTimeout:  30 * time.Second,
			}

			// Graceful shutdown
			done := make(chan os.Signal, 1)
			signal.Notify(done, os.Interrupt, syscall.SIGTERM)

			go func() {
				<-done
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				srv.Shutdown(ctx)
			}()

			fmt.Printf("Aegis API server starting on %s\n", addr)
			fmt.Printf("Auth: Bearer token required\n")
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("server error: %v", err)
			}
			fmt.Println("Server stopped.")
			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "", "Listen address (default: 127.0.0.1:7380)")
	return cmd
}
