package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

type benchJob struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Code     string `json:"code"`
}

type agentExecReq struct {
	ID      string `json:"id"`
	Command string `json:"command"`
}

type agentRunReq struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Code     string `json:"code"`
	Variant  string `json:"variant"`
}

type agentExecRes struct {
	Message      string `json:"message"`
	Error        string `json:"error"`
	StdErr       string `json:"stderr"`
	StdOut       string `json:"stdout"`
	ExecDuration int    `json:"exec_duration"`
	MemUsage     int64  `json:"mem_usage"`
}

type runningFirecracker struct {
	vmmCtx    context.Context
	vmmCancel context.CancelFunc
	machine   *firecracker.Machine
	ip        net.IP
}

var (
	q jobQueue
)

func main() {
	defer deleteVMMSockets()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	WarmVMs := make(chan runningFirecracker, 1)

	go fillVMPool(ctx, WarmVMs)
	installSignalHandlers()
	log.SetReportCaller(true)

	rabbitMQURL := os.Getenv("RABBITMQ_URL")
	if len(rabbitMQURL) == 0 {
		logrus.Fatal("Missing RABBITMQ_URL env variable")
	}
	q = newJobQueue(rabbitMQURL)
	defer q.ch.Close()
	defer q.conn.Close()

	err := q.getQueueForJob(ctx)
	if err != nil {
		log.WithError(err).Fatal("Failed to get status queue")
		return
	}

	log.Info("Waiting for RabbitMQ jobs...")
	for d := range q.jobs {
		log.Printf("Received a message: %s", d.Body)

		var job benchJob
		err := json.Unmarshal([]byte(d.Body), &job)
		if err != nil {
			log.WithError(err).Error("Received invalid job")
			continue
		}

		go job.run(ctx, WarmVMs)
	}
}

// TODO this isn't called for whatever reason
func deleteVMMSockets() {
	log.Debug("cc")
	dir, err := ioutil.ReadDir(os.TempDir())
	if err != nil {
		log.WithError(err).Error("Failed to read directory")
	}
	for _, d := range dir {
		log.WithField("d", d.Name()).Debug("considering")
		if strings.Contains(d.Name(), fmt.Sprintf(".firecracker.sock-%d-", os.Getpid())) {
			log.WithField("d", d.Name()).Debug("should delete")
			os.Remove(path.Join([]string{"tmp", d.Name()}...))
		}
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
				deleteVMMSockets()
				os.Exit(0)
			case s == syscall.SIGQUIT:
				log.Printf("Caught signal: %s, forcing shutdown", s.String())
				deleteVMMSockets()
				os.Exit(0)
			}
		}
	}()
}
