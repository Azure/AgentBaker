package main

// Version is set at build time via -ldflags "-X main.Version=<version>".
// It defaults to "dev" for local development builds.
var Version = "dev"
