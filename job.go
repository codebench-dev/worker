package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
)

func (job benchJob) run(ctx context.Context, WarmVMs <-chan runningFirecracker) {
	log.WithField("job", job).Info("Handling job")

	err := q.getQueueForJob(ctx, job)
	if err != nil {
		log.WithError(err).Fatal("Failed to get status queue")
		return
	}

	err = q.setjobReceived(ctx, job)
	if err != nil {
		q.setjobFailed(ctx, job)
		return
	}

	// Get a ready-to-use microVM from the pool
	vm := <-WarmVMs

	// Defer cleanup of VM and VMM
	go func() {
		defer vm.vmmCancel()
		vm.machine.Wait(vm.vmmCtx)
	}()
	defer vm.shutDown()

	var reqJSON []byte
	switch job.Type {
	case "command":
		reqJSON, err = json.Marshal(agentExecReq{Command: job.Command})
		if err != nil {
			log.WithError(err).Error("Failed to marshal JSON request")
			q.setjobFailed(ctx, job)
			return
		}
	case "code":
		reqJSON, err = json.Marshal(agentRunReq{Code: job.Code})
		if err != nil {
			log.WithError(err).Error("Failed to marshal JSON request")
			q.setjobFailed(ctx, job)
			return
		}
	}

	err = q.setjobRunning(ctx, job)
	if err != nil {
		q.setjobFailed(ctx, job)
		return
	}

	var httpRes *http.Response
	var res agentExecRes

	switch job.Type {
	case "command":
		httpRes, err = http.Post("http://"+vm.ip.String()+":8080/exec", "application/json", bytes.NewBuffer(reqJSON))
		if err != nil || httpRes.StatusCode != 200 {
			log.WithError(err).Error("Failed to request execution to agent")
			q.setjobFailed(ctx, job)
			return
		}
		json.NewDecoder(httpRes.Body).Decode(&res)
		log.WithField("result", res).Info("Job execution finished")

		err = q.setjobResult(ctx, job, res)
		if err != nil {
			q.setjobFailed(ctx, job)
		}

	case "code":
		httpRes, err = http.Post("http://"+vm.ip.String()+":8080/run/c", "application/json", bytes.NewBuffer(reqJSON))
		if err != nil {
			log.WithError(err).Error("Failed to request execution to agent")
			q.setjobFailed(ctx, job)
			return
		}
		json.NewDecoder(httpRes.Body).Decode(&res)
		log.WithField("result", res).Info("Job execution finished")

		if httpRes.StatusCode != 200 {
			log.WithField("res", res).Error("Failed to compile and run code")
			q.setjobFailed(ctx, job)
			return
		}

		err = q.setjobResult(ctx, job, res)
		if err != nil {
			q.setjobFailed(ctx, job)
		}
	}

}
