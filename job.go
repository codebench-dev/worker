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

	res, err := q.setjobReceived(ctx, job)
	if err != nil && res != 1 {
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

	reqJSON, err := json.Marshal(agentExecReq{Command: job.Command})
	if err != nil {
		log.WithError(err).Error("Failed to marshal JSON request")
		q.setjobFailed(ctx, job)
		return
	}

	res, err = q.setjobRunning(ctx, job)
	if err != nil && res != 1 {
		q.setjobFailed(ctx, job)
		return
	}

	var agentRes agentExecRes
	httpRes, err := http.Post("http://"+vm.ip.String()+":8080/exec", "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		log.WithError(err).Error("Failed to request execution to agent")
		q.setjobFailed(ctx, job)
		return
	}

	json.NewDecoder(httpRes.Body).Decode(&agentRes)
	log.WithField("result", agentRes).Info("Job execution finished")

	res, err = q.setjobResult(ctx, job, agentRes)
	if err != nil && res != 1 {
		q.setjobFailed(ctx, job)
	}
}
