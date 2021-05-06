package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

type benchJob struct {
	Id      string `json:"id"`
	Command string `json:"command"`
}

type agentExecReq struct {
	Command string `json:"command"`
}

type agentExecRes struct {
	Command string `json:"command"`
	StdErr  string `json:"stderr"`
	StdOut  string `json:"stdout"`
}

type RunningFirecracker struct {
	ctx       context.Context
	cancelCtx context.CancelFunc
	machine   *firecracker.Machine
	ip        net.IP
}

var (
	runningVMs map[string]RunningFirecracker = make(map[string]RunningFirecracker)
	q          jobQueue
)

func main() {
	defer cleanup()
	installSignalHandlers()

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6378",
	})

	q = jobQueue{redis: redisClient}

	fmt.Println("Waiting for jobs on redis job queue")

	for {
		var job benchJob
		result, err := q.getJob()

		if err != nil {
			log.WithError(err).Error("Failed to get job from redis queue")
			continue
		}
		json.Unmarshal([]byte(result[1]), &job)

		go job.run()
	}
}

func installSignalHandlers() {
	go func() {
		// Clear some default handlers installed by the firecracker SDK:
		signal.Reset(os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

		for {
			switch s := <-c; {
			case s == syscall.SIGTERM || s == os.Interrupt:
				log.Printf("Caught signal: %s, requesting clean shutdown", s.String())
				cleanup()
				os.Exit(0)
			case s == syscall.SIGQUIT:
				log.Printf("Caught signal: %s, forcing shutdown", s.String())
				cleanup()
				os.Exit(0)
			}
		}
	}()
}
