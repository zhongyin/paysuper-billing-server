package main

import (
	"github.com/ProtocolONE/payone-billing-service/internal"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	app := internal.NewApplication()
	app.Init()

	go handleOsSignals(app)

	app.Run()
}

func handleOsSignals(app *internal.Application) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	exitChan := make(chan int)

	go func() {
		for {
			s := <-signalChan
			switch s {
			case os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT:
				app.Stop()
				exitChan <- 0
			}
		}
	}()

	<-exitChan
	return
}
