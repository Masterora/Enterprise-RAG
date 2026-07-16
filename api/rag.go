// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"enterprise-rag/api/internal/config"
	"enterprise-rag/api/internal/handler"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/transport/internaltools"
	"enterprise-rag/api/internal/worker"

	"github.com/zeromicro/go-zero/core/trace"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/rag-api.yaml", "the config file")

func main() {
	flag.Parse()

	c, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()
	defer trace.StopAgent()

	ctx, err := svc.NewServiceContext(c)
	if err != nil {
		log.Fatalf("initialize service context: %v", err)
	}
	defer ctx.Close()
	if ctx.Metrics != nil {
		server.Use(ctx.Metrics.HTTPMiddleware(c.Metrics.Path))
		server.AddRoute(rest.Route{Method: http.MethodGet, Path: c.Metrics.Path, Handler: ctx.Metrics.Handler().ServeHTTP})
	}
	server.AddRoute(rest.Route{
		Method: http.MethodGet,
		Path:   "/healthz",
		Handler: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		},
	})
	server.AddRoute(rest.Route{
		Method: http.MethodGet,
		Path:   "/readyz",
		Handler: func(w http.ResponseWriter, request *http.Request) {
			checkContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
			defer cancel()
			w.Header().Set("Content-Type", "application/json")
			if err := ctx.Ready(checkContext); err != nil {
				log.Printf("readiness check failed: %v", err)
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"not_ready"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ready"}`))
		},
	})
	workerManager, err := worker.NewManager(ctx)
	if err != nil {
		log.Fatalf("initialize worker manager: %v", err)
	}
	if err := workerManager.Start(); err != nil {
		log.Fatalf("start workers: %v", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(
			context.Background(),
			time.Duration(c.Worker.ShutdownTimeoutSeconds)*time.Second,
		)
		defer shutdownCancel()
		if err := workerManager.Close(shutdownCtx); err != nil {
			log.Printf("stop workers: %v", err)
		}
	}()
	handler.RegisterHandlers(server, ctx)
	internaltools.RegisterRoutes(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
