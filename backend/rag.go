// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/handler"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/worker"

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
	workerManager, err := worker.NewManager(ctx)
	if err != nil {
		log.Fatalf("initialize worker manager: %v", err)
	}
	if err := workerManager.Start(); err != nil {
		log.Fatalf("start workers: %v", err)
	}
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
