package main

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

// loadEnvFile loads .env when it exists. Variables already set in the
// environment are left alone. A missing file is not an error.
func loadEnvFile() error {
	err := godotenv.Load()
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
