/*
Copyright © 2022 Hanko GmbH <developers@hanko.io>
*/
package main

import (
	"github.com/getsentry/sentry-go"
	"github.com/teamhanko/hanko/backend/cmd"
	"log"
	"os"
	"time"
)

func main() {
	dsn := os.Getenv("SENTRY_DSN")

	if dsn != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn: dsn,
		})
		if err != nil {
			log.Fatalf("sentry.Init: %s", err)
		}
		// Flush buffered events before the program terminates.
		// Set the timeout to the maximum duration the program can afford to wait.
		defer sentry.Flush(2 * time.Second)
	}

	cmd.Execute()
}
