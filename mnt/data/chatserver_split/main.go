package main

import "log"

// main is intentionally small so startup flow is easy to follow.
func main() {
	if err := run(); err != nil {
		log.Fatalf("server startup failed: %v", err)
	}
}
