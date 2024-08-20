package main

import (
	"log/slog"
	"os"
)

func main() {
	file, err := os.OpenFile("installer.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	slog.SetDefault(slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{AddSource: true})))
	slog.Info("Starting installer")
}
