package main

import (
	"Threadly/internal/loadtest"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		invalidInput()
	}
	if os.Args[1] == "report" {
		os.Exit(report(os.Args[2:]))
	}
	if os.Args[1] != "plan" {
		invalidInput()
	}
	flags := flag.NewFlagSet("plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	scenario := flags.String("scenario", "", "")
	dataset := flags.String("dataset", "", "")
	if flags.Parse(os.Args[2:]) != nil || flags.NArg() != 0 {
		invalidInput()
	}
	plan, err := loadtest.NewPlan(*scenario, *dataset)
	if err != nil {
		invalidInput()
	}
	if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
		os.Exit(5)
	}
}

func report(args []string) int {
	flags := flag.NewFlagSet("report", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	manifestPath := flags.String("manifest", "", "")
	pointsPath := flags.String("points", "", "")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *manifestPath == "" || *pointsPath == "" {
		invalidInput()
	}
	manifest, err := os.Open(*manifestPath)
	if err != nil {
		invalidInput()
	}
	defer manifest.Close()
	points, err := os.Open(*pointsPath)
	if err != nil {
		invalidInput()
	}
	defer points.Close()
	result, err := loadtest.GenerateReport(manifest, points)
	if err != nil {
		invalidInput()
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return 5
	}
	return result.ExitCode
}

func invalidInput() { fmt.Fprintln(os.Stderr, "loadtest invalid_input"); os.Exit(5) }
