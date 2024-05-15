package main

import (
	"fmt"
	"os"
	"os/exec"
)

// this needs to get uploaded onto the VM to execute

func execute_scans() {
	// Create directory if not exists
	err := os.MkdirAll("/mnt/sda1", 0755)
	if err != nil {
		fmt.Println("Error creating directory:", err)
		return
	}

	// Mount device to directory
	mountCmd := exec.Command("mount", "/dev/sda1", "/mnt/sda1")
	mountCmd.Stdout = os.Stdout
	mountCmd.Stderr = os.Stderr
	if err := mountCmd.Run(); err != nil {
		fmt.Println("Error mounting device:", err)
		return
	}

	// Execute storage-scan.sh
	if err := runScript("/home/packer/storage-scan.sh"); err != nil {
		fmt.Println("Error running storage-scan.sh:", err)
		// Optionally, you may choose to continue or abort based on your needs
	}

	// Execute trivy-scan.sh
	if err := runScript("/home/packer/trivy-scan.sh"); err != nil {
		fmt.Println("Error running trivy-scan.sh:", err)
		// Optionally, you may choose to continue or abort based on your needs
	}

	// Unmount directory
	umountCmd := exec.Command("umount", "/mnt/sda1")
	umountCmd.Stdout = os.Stdout
	umountCmd.Stderr = os.Stderr
	if err := umountCmd.Run(); err != nil {
		fmt.Println("Error unmounting device:", err)
		return
	}

	// Remove directory
	if err := os.Remove("/mnt/sda1"); err != nil {
		fmt.Println("Error removing directory:", err)
		return
	}

	// upload files to blob storage
}

func runScript(scriptPath string) error {
	cmd := exec.Command("bash", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
