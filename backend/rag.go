// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/handler"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/worker"

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

	ctx := svc.NewServiceContext(c)
	workerManager, err := worker.NewManager(context.Background(), ctx)
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
