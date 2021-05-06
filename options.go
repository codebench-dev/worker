package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
)

// Converts options to a usable firecracker config
func getFirecrackerConfig() (firecracker.Config, error) {
	return firecracker.Config{
		SocketPath:      getSocketPath(),
		KernelImagePath: "../../linux/vmlinux",
		Drives: []models.Drive{{
			DriveID: firecracker.String("1"),
			// TODO: copy base rootfs and use a temp roots per VM
			PathOnHost:   firecracker.String("../agent/rootfs.ext4"),
			IsRootDevice: firecracker.Bool(true),
			IsReadOnly:   firecracker.Bool(false),
		}},
		NetworkInterfaces: []firecracker.NetworkInterface{{
			// Use CNI to get dynamic IP
			CNIConfiguration: &firecracker.CNIConfiguration{
				NetworkName: "fcnet",
				IfName:      "veth0",
			},
		}},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(2),
			HtEnabled:  firecracker.Bool(true),
			MemSizeMib: firecracker.Int64(512),
		},
	}, nil
}

func getSocketPath() string {
	filename := strings.Join([]string{
		".firecracker.sock",
		strconv.Itoa(os.Getpid()),
		strconv.Itoa(rand.Intn(1000))},
		"-",
	)
	dir := os.TempDir()

	return filepath.Join(dir, filename)
}
