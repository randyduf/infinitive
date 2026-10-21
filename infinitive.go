package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/acd/infinitive/infinity"
	"github.com/acd/infinitive/internal/probes"
	log "github.com/sirupsen/logrus"
)

func main() {
	httpPort := flag.Int("httpport", 8080, "HTTP port to listen on")
	serialPort := flag.String("serial", "", "path to serial port")
	enableProbes := flag.Bool("probes", true, "read DS18B20 temperature probes from the 1-Wire bus")
	probeTop := flag.String("probe-top", "", "1-Wire ID of the Wine (top) probe, e.g. 28-0316a2795e3b (default: first sensor found)")
	probeBottom := flag.String("probe-bottom", "", "1-Wire ID of the Wine (bottom) probe (default: second sensor found)")

	flag.Parse()

	if len(*serialPort) == 0 {
		fmt.Print("must provide serial\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	log.SetLevel(log.DebugLevel)

	infinityApi, err := infinity.NewApi(context.Background(), *serialPort)
	if err != nil {
		log.Panicf("error opening serial port: %s", err.Error())
	}

	if *enableProbes {
		monitor := probes.NewMonitor(infinityApi.Cache, probes.Config{
			TopID:    *probeTop,
			BottomID: *probeBottom,
		})
		go monitor.Run(context.Background())
	}

	launchWebserver(*httpPort, infinityApi)
}
