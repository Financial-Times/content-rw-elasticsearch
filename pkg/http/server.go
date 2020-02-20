package http

import (
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Financial-Times/go-logger/v2"
)

func StartServer(log *logger.UPPLogger, serveMux *http.ServeMux, port string) {
	server := &http.Server{Addr: ":" + port, Handler: serveMux}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.WithError(err).Error("HTTP server is closing")
		}
		wg.Done()
	}()

	waitForSignal()
	log.Info("[Shutdown] Application is shutting down")

	if err := server.Close(); err != nil {
		log.WithError(err).Error("Unable to stop http server")
	}
	wg.Wait()
}

func waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
