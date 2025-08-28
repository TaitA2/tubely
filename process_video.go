package main

import (
	"fmt"
	"os/exec"
)

func processVideoForFastStart(filePath string) (string, error) {
	outputPath := filePath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("Error running ffmpeg: %v", err)
	}
	return outputPath, nil

}
