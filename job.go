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
		log.WithError(err).Error("Could not set job received")
		q.setjobFailed(ctx, job, agentExecRes{Error: err.Error()})
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
		ID:       job.ID,
		Language: job.Language,
		Code:     job.Code,
		Variant:  "TODO",
	})
	if err != nil {
		log.WithError(err).Error("Failed to marshal JSON request")
		q.setjobFailed(ctx, job, agentExecRes{Error: err.Error()})
		return
	}

	err = q.setjobRunning(ctx, job)
	if err != nil {
		log.WithError(err).Error("Could not set job running")
		q.setjobFailed(ctx, job, agentExecRes{Error: err.Error()})
		return
	}

	var httpRes *http.Response
	var agentRes agentExecRes

	httpRes, err = http.Post("http://"+vm.ip.String()+":8080/run", "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		log.WithError(err).Error("Failed to request execution to agent")
		q.setjobFailed(ctx, job, agentExecRes{Error: err.Error()})
		return
	}
	json.NewDecoder(httpRes.Body).Decode(&agentRes)
	log.WithField("result", agentRes).Info("Job execution finished")
	if httpRes.StatusCode != 200 {
		log.WithFields(log.Fields{
			"httpRes":  httpRes,
			"agentRes": agentRes,
			"reqJSON":  string(reqJSON),
		}).Error("Failed to compile and run code")
		q.setjobFailed(ctx, job, agentRes)
		return
	}

	err = q.setjobResult(ctx, job, agentRes)
	if err != nil {
		q.setjobFailed(ctx, job, agentExecRes{Error: err.Error()})
	}
}
