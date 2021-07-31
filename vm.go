package main

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/imroc/req"
	log "github.com/sirupsen/logrus"
)

func waitForVMToBoot(ctx context.Context, ip net.IP) error {
	// Query the agent until it provides a valid response
	req.SetTimeout(500 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			// Timeout
			return ctx.Err()
		default:
			res, err := req.Get("http://" + ip.String() + ":8080/health")
			if err != nil {
				log.WithError(err).Info("VM not ready yet")
				time.Sleep(time.Second)
				continue
			}

			if res.Response().StatusCode != 200 {
				time.Sleep(time.Second)
				log.Info("VM not ready yet")
			} else {
				log.WithField("ip", ip).Info("VM agent ready")
				return nil
			}
			time.Sleep(time.Second)
		}

	}
}

func (vm runningFirecracker) shutDown() {
	log.WithField("ip", vm.ip).Info("stopping")
	vm.machine.StopVMM()
	err := os.Remove(vm.machine.Cfg.SocketPath)
	if err != nil {
		log.WithError(err).Error("Failed to delete firecracker socket")
	}
	err = os.Remove("/tmp/rootfs-" + vm.vmmID + ".ext4")
	if err != nil {
		log.WithError(err).Error("Failed to delete firecracker rootfs")
	}
}
