package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"armada/internal/gpio"
	"armada/internal/host"
	"armada/internal/httpapi"
)

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address")
	noGPIO := flag.Bool("no-gpio", false, "disable sysfs GPIO (stub backend)")
	gpioState := flag.String("gpio-state", "/var/lib/armada/gpio.json", "GPIO state file path")
	iomuxDev := flag.String("iomux-dev", "/dev/iomux", "Rockchip pinmux device (empty to disable auto mux to GPIO)")
	flag.Parse()

	gpio.SetIomuxDevice(*iomuxDev)

	collector := host.NewCollector()
	collector.Start(time.Second)

	gpioMgr := gpio.NewManager(gpio.NewStore(*gpioState), *noGPIO)
	srv := httpapi.New(collector, gpioMgr)
	log.Printf("armada listening on %s", *listen)
	if err := http.ListenAndServe(*listen, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
