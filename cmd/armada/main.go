package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"armada/internal/host"
	"armada/internal/httpapi"
)

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address")
	flag.Parse()

	collector := host.NewCollector()
	collector.Start(time.Second)

	srv := httpapi.New(collector)
	log.Printf("armada listening on %s", *listen)
	if err := http.ListenAndServe(*listen, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
