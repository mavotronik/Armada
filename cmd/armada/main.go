package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"armada/internal/gpio"
	"armada/internal/host"
	"armada/internal/httpapi"
	"armada/internal/settings"
)

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address")
	dbPath := flag.String("db-path", "/etc/armada", "directory for SQLite databases")
	noGPIO := flag.Bool("no-gpio", false, "disable sysfs GPIO (stub backend)")
	iomuxDev := flag.String("iomux-dev", "/dev/iomux", "Rockchip pinmux device (empty to disable auto mux to GPIO)")
	flag.Parse()

	gpio.SetIomuxDevice(*iomuxDev)

	settingsStore, err := settings.Open(*dbPath)
	if err != nil {
		log.Fatalf("settings db: %v", err)
	}
	defer settingsStore.Close()

	gpioStore, err := gpio.NewStore(*dbPath)
	if err != nil {
		log.Fatalf("gpio db: %v", err)
	}

	collector := host.NewCollector()
	collector.Start(time.Second)

	gpioMgr := gpio.NewManager(gpioStore, *noGPIO)
	srv := httpapi.New(collector, gpioMgr, settingsStore)
	log.Printf("armada listening on %s", *listen)
	if err := http.ListenAndServe(*listen, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
