package main

import (
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

func waitForVMToBoot(net.IP) {
	// TODO ping agent until alive
	time.Sleep(5 * time.Second)
}

func (vm RunningFirecracker) shutDown() {
	log.WithField("ip", vm.ip).Info("stopping")
	vm.machine.StopVMM()
}

func cleanup() {
	for _, vm := range runningVMs {
		vm.shutDown()
	}
}
