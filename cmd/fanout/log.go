package main

import (
	"log/slog"
	"os"
)

var (
	accessLogger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	errorLogger  = slog.Default()
)
