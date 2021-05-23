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
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

type benchJob struct {
	ID      string `json:"id"`
	Variant string `json:"variant"`
	Code    string `json:"code"`
}

type agentExecReq struct {
	ID      string `json:"id"`
	Command string `json:"command"`
}

type agentRunReq struct {
	ID      string `json:"id"`
	Variant string `json:"variant"`
	Code    string `json:"code"`
}

type agentExecRes struct {
	StdErr string `json:"stderr"`
	StdOut string `json:"stdout"`
}

type runningFirecracker struct {
	vmmCtx    context.Context
	vmmCancel context.CancelFunc
	machine   *firecracker.Machine
	ip        net.IP
}

var (
	q jobQueueRedis
)

func main() {
	defer deleteVMMSockets()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	WarmVMs := make(chan runningFirecracker, 0)

	go fillVMPool(ctx, WarmVMs)
	installSignalHandlers()
	log.SetReportCaller(true)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6378",
	})

	q = jobQueueRedis{redis: redisClient}

	fmt.Println("Waiting for jobs on redis job queue")

	for {
		var job benchJob
		result, err := q.getJob(ctx)

		if err != nil {
			log.WithError(err).Error("Failed to get job from redis queue")
			continue
		}
		err = json.Unmarshal([]byte(result[1]), &job)
		if err != nil || job.ID == "" || job.Code == "" || job.Variant == "" {
			log.WithError(err).WithField("job", result[1]).Error("Failed to unmarshal job")
			q.setjobFailed(ctx, job)
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
