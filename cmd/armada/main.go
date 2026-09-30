package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	"armada/internal/cluster"
	"armada/internal/host"
	"armada/internal/httpapi"
	"armada/internal/uart"
)

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address")
	nodeUART := flag.String("node-uart", "", "serial device for the node metrics agent")
	nodeBaud := flag.Int("node-baud", 9600, "baud for -node-uart; must match NODE_BAUD in the hub sketch")
	hubDev := flag.String("hub", "", "serial device of the Pro Mini UART hub")
	hubBaud := flag.Int("hub-baud", 115200, "baud for -hub")
	hubNodes := flag.Int("nodes", 4, "nodes on the hub (1..6); must match NUM_NODES in the hub sketch")
	flag.Parse()

	if *nodeUART != "" && *hubDev != "" {
		log.Fatal("-node-uart and -hub are different roles")
	}

	collector := host.NewCollector()
	collector.Start(time.Second)

	if *nodeUART != "" {
		runNode(*nodeUART, *nodeBaud, collector)
		return
	}

	var board *cluster.Board
	if *hubDev != "" {
		board = runHub(*hubDev, *hubBaud, *hubNodes)
	}

	srv := httpapi.New(collector, board)
	log.Printf("armada listening on %s", *listen)
	if err := http.ListenAndServe(*listen, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}

func runNode(path string, baud int, collector *host.Collector) {
	port, err := uart.Open(path, baud)
	if err != nil {
		log.Fatal(err)
	}
	defer port.Close()
	log.Printf("node uart agent on %s at %d", path, baud)
	if err := uart.Serve(port, collector.Snapshot, uart.Exec); err != nil {
		log.Fatal(err)
	}
}

func runHub(path string, baud, nodes int) *cluster.Board {
	if nodes < 1 || nodes > 6 {
		log.Fatal("-nodes must be from 1 to 6")
	}
	port, err := uart.Open(path, baud)
	if err != nil {
		log.Fatal(err)
	}
	client := uart.NewClient(port, nodes)
	if err := client.Ping(); err != nil {
		log.Printf("uart hub: no answer yet on %s: %v", path, err)
	} else {
		log.Printf("uart hub ready on %s, %d nodes", path, nodes)
	}
	board := cluster.NewBoard(client, 5*time.Second)
	go board.Run(context.Background())
	return board
}
