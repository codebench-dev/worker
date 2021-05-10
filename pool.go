package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

func fillVMPool(ctx context.Context, WarmVMs chan<- runningFirecracker) {
	for {
		select {
		case <-ctx.Done():
			// Program is stopping, WarmVMs will be cleaned up, bye
			return
		default:
			vm, err := createAndStartVM(ctx)
			if err != nil {
				log.Error("failed to create VMM")
				time.Sleep(time.Second)
				continue
			}
			log.WithField("ip", vm.ip).Info("New VM created and started")
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			err = waitForVMToBoot(ctx, vm.ip)
			if err != nil {
				log.WithError(err).Info("VM not ready yet")
				vm.vmmCancel()
				continue
			}
			WarmVMs <- *vm
		}
	}
}
