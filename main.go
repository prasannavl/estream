package main

import (
	"bufio"
	"context"
	"fmt"

	"github.com/prasannavl/go-gluons/appx"

	"net/http"
	"os"

	"io/ioutil"

	"github.com/antage/eventsource"
	log "github.com/prasannavl/go-gluons/log"
	logc "github.com/prasannavl/go-gluons/logconfig"
	flag "github.com/spf13/pflag"
)

func main() {
	var verbosity int
	var logFile string
	var logDisabled bool
	var pipeInOut bool
	var addr string

	flag.Usage = func() {
		fmt.Printf("\nUsage: [opts]\n\nOptions:\n")
		flag.PrintDefaults()
		fmt.Println()
	}

	flag.CountVarP(&verbosity, "verbose", "v", "verbosity level")
	flag.StringVar(&logFile, "log", "", "the log file destination")
	flag.BoolVar(&logDisabled, "no-log", false, "disable the logger")
	flag.BoolVar(&pipeInOut, "pipe", false, "output the stdin back to stdout")
	flag.StringVarP(&addr, "address", "a", "localhost:3000",
		"the 'host:port' for the service to listen on")
	flag.Parse()

	logInitResult := logc.LogInitResult{}
	if !logDisabled {
		logOpts := logc.DefaultOptions()
		if logFile != "" {
			logOpts.LogFile = logFile
		}
		logOpts.VerbosityLevel = verbosity
		logc.Init(&logOpts, &logInitResult)
	}

	log.Infof("listen address: %q", addr)

	run(addr, pipeInOut)
}

func run(addr string, pipeInOut bool) {
	es := eventsource.New(
		eventsource.DefaultSettings(),
		func(r *http.Request) [][]byte {
			return [][]byte{
				[]byte("Access-Control-Allow-Origin: *"),
			}
		})

	mux := http.NewServeMux()
	mux.HandleFunc("/in", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		res, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), 400)
		}
		sendEventMessage(es, string(res), "", "")
	})
	mux.Handle("/", es)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	appx.CreateShutdownHandler(func() {
		es.Close()
		httpServer.Shutdown(context.Background())
	}, appx.ShutdownSignals...)

	go runMessageProcessor(es, pipeInOut)

	if err := httpServer.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			log.Error(err.Error())
			os.Exit(1)
		}
	}

	log.Info("exit")
}

func runMessageProcessor(es eventsource.EventSource, pipeInOut bool) {
	scanner := bufio.NewScanner(os.Stdin)
	log.Info("start stdin processor")
	for scanner.Scan() {
		t := scanner.Text()
		if pipeInOut {
			os.Stdout.WriteString(t + "\r\n")
		}
		sendEventMessage(es, t, "", "")
	}
	log.Info("end stdin processor")
}

func sendEventMessage(es eventsource.EventSource, msg string, event string, id string) {
	log.Tracef("publish: %v", msg)
	es.SendEventMessage(msg, event, id)
}
