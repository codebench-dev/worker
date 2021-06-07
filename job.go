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

	err := q.setjobReceived(ctx, job)
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

	reqJSON, err = json.Marshal(agentRunReq{
		ID:      job.ID,
		Variant: job.Variant,
		Code:    job.Code,
	})
	if err != nil {
		log.WithError(err).Error("Failed to marshal JSON request")
		q.setjobFailed(ctx, job)
		return
	}

	err = q.setjobRunning(ctx, job)
	if err != nil {
		q.setjobFailed(ctx, job)
		return
	}

	var httpRes *http.Response
	var agentRes agentExecRes

	// FIXME
	httpRes, err = http.Post("http://"+vm.ip.String()+":8080/run/python", "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		log.WithError(err).Error("Failed to request execution to agent")
		q.setjobFailed(ctx, job)
		return
	}
	json.NewDecoder(httpRes.Body).Decode(&agentRes)
	log.WithField("result", agentRes).Info("Job execution finished")
	if httpRes.StatusCode != 200 {
		log.WithField("res", agentRes).Error("Failed to compile and run code")
		q.setjobFailed(ctx, job)
		return
	}

	err = q.setjobResult(ctx, job, agentRes)
	if err != nil {
		q.setjobFailed(ctx, job)
	}

}
