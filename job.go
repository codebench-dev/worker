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

	statusQueue, err := q.getQueueForJob(ctx, job)
	if err != nil {
		log.WithError(err).Fatal("Failed to get status queue")
		return
	}

	err = q.setjobReceived(ctx, statusQueue, job)
	if err != nil {
		q.setjobFailed(ctx, statusQueue, job)
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

	reqJSON, err := json.Marshal(agentExecReq{Command: job.Command})
	if err != nil {
		log.WithError(err).Error("Failed to marshal JSON request")
		q.setjobFailed(ctx, statusQueue, job)
		return
	}

	err = q.setjobRunning(ctx, statusQueue, job)
	if err != nil {
		q.setjobFailed(ctx, statusQueue, job)
		return
	}

	var agentRes agentExecRes
	httpRes, err := http.Post("http://"+vm.ip.String()+":8080/exec", "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		log.WithError(err).Error("Failed to request execution to agent")
		q.setjobFailed(ctx, statusQueue, job)
		return
	}

	json.NewDecoder(httpRes.Body).Decode(&agentRes)
	log.WithField("result", agentRes).Info("Job execution finished")

	err = q.setjobResult(ctx, statusQueue, job, agentRes)
	if err != nil {
		q.setjobFailed(ctx, statusQueue, job)
	}
}
