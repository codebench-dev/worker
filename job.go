package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
)

func (job benchJob) run() {
	log.WithField("job", job).Info("Handling job")

	res, err := q.setjobReceived(job)
	if err != nil && res != 1 {
		q.setjobFailed(job)
		return
	}

	vm, err := createVMM(context.Background())
	if err != nil {
		log.Error("failed to creat VMM")
		q.setjobFailed(job)
		return
	}

	// store in global map to be able to cleanup from outside the goroutine
	runningVMs[vm.ip.String()] = *vm

	go func() {
		defer vm.cancelCtx()
		vm.machine.Wait(vm.ctx)
	}()

	defer vm.shutDown()
	defer delete(runningVMs, vm.ip.String())

	waitForVMToBoot(vm.ip)

	json_data, err := json.Marshal(agentExecReq{Command: job.Command})
	if err != nil {
		log.WithError(err).Error("Failed to marshal JSON request")
		q.setjobFailed(job)
		return
	}

	res, err = q.setjobRunning(job)
	if err != nil && res != 1 {
		q.setjobFailed(job)
		return
	}

	var agentRes agentExecRes
	httpRes, err := http.Post("http://"+vm.ip.String()+":8080/exec", "application/json", bytes.NewBuffer(json_data))
	if err != nil {
		log.WithError(err).Error("Failed to request execution to agent")
		q.setjobFailed(job)
		return
	}

	json.NewDecoder(httpRes.Body).Decode(&agentRes)
	log.WithField("result", agentRes).Info("Job execution finished")

	res, err = q.setjobResult(job, agentRes)
	if err != nil && res != 1 {
		q.setjobFailed(job)
	}
}
