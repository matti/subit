package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

type event struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func printer(queue chan event) {
	for {
		msg, err := json.Marshal(<-queue)
		if err != nil {
			panic(err)
		}

		fmt.Println(string(msg))
	}
}

func waitForever(done chan bool) {
	<-done
	log.Println("bye")

	os.Exit(0)
}

func main() {
	var queue = make(chan event, 100)
	go printer(queue)

	queue <- event{
		Type:    "pid",
		Message: fmt.Sprint(os.Getpid()),
	}

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs

		queue <- event{
			Type:    "signal",
			Message: sig.String(),
		}

		done <- true
	}()

	natsServers := os.Args[1]
	natsPingInterval, err := time.ParseDuration(os.Args[2])
	if err != nil {
		log.Fatalln("ParseDuration", err)
	}
	natsSubject := os.Args[3]

	nc, err := nats.Connect(natsServers,
		nats.NoReconnect(),
		nats.MaxPingsOutstanding(1),
		nats.PingInterval(natsPingInterval),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			queue <- event{
				Type:    "disconnected",
				Message: nc.ConnectedAddr(),
			}
		}),
	)

	if err != nil {
		queue <- event{
			Type: "connectError",
		}
		waitForever(done)
	}

	queue <- event{
		Type:    "connected",
		Message: nc.ConnectedAddr(),
	}

	nc.Subscribe("test", func(m *nats.Msg) {
		queue <- event{
			Type:    "message",
			Message: string(m.Data),
		}
	})

	queue <- event{
		Type:    "subscribed",
		Message: natsSubject,
	}

	waitForever(done)
}
