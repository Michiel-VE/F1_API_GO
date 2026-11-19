//go:build ignore

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
)

func main() {
	year := flag.String("year", "", "Season year to backfill (e.g., 2021)")
	flag.Parse()

	if *year == "" {
		log.Fatal("Please provide a year using -year (e.g., go run scripts/backfill.go -year=2021)")
	}

	steps := []struct {
		label string
		mode  string
	}{
		{"1. Scraping Teams...", "scraper-driver-team"},
		{"2. Scraping Drivers...", "scraper-driver"},
		{"3. Scraping Races...", "scraper-race"},
		{"4. Scraping Standings...", "scraper-standing"},
		{"5. Scraping Race Results...", "scraper-race-result"},
	}

	for _, step := range steps {
		fmt.Println(step.label)

		cmd := exec.Command("go", "run", "main.go")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(),
			"RUN_YEAR="+*year,
			"RUN_MODE="+step.mode,
			"BACKFILL=true",
		)

		if err := cmd.Run(); err != nil {
			log.Fatalf("Backfill aborted at %s (%s): %v", step.label, step.mode, err)
		}
	}

	fmt.Printf("Backfill for %s completed successfully!\n", *year)
}
