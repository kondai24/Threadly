package main

import (
	"Threadly/internal/loadtest"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		invalidInput()
	}
	switch os.Args[1] {
	case "report":
		os.Exit(report(os.Args[2:]))
	case "prepare":
		os.Exit(prepare(os.Args[2:]))
	case "reset":
		os.Exit(reset(os.Args[2:]))
	case "seed":
		os.Exit(seed(os.Args[2:]))
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
	defer func() { _ = manifest.Close() }()
	points, err := os.Open(*pointsPath)
	if err != nil {
		invalidInput()
	}
	defer func() { _ = points.Close() }()
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

func prepare(args []string) int {
	flags := flag.NewFlagSet("prepare", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dataset := flags.String("dataset", "", "")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		invalidInput()
	}
	root, err := os.Getwd()
	if err != nil {
		return fail(&loadtest.Failure{Code: 4, Reason: "environment_unavailable"})
	}
	summary, err := loadtest.Environment{Root: root}.Prepare(context.Background(), *dataset)
	if err != nil {
		return fail(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil {
		return 5
	}
	return 0
}

// failは、固定分類だけをstderrへ出して対応する終了コードを返す。
func fail(err error) int {
	var failure *loadtest.Failure
	if !errors.As(err, &failure) {
		failure = &loadtest.Failure{Code: 4, Reason: "environment_unavailable"}
	}
	fmt.Fprintln(os.Stderr, "loadtest "+failure.Reason)
	return failure.Code
}

func reset(args []string) int {
	flags := flag.NewFlagSet("reset", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	confirm := flags.String("confirm-project", "", "")
	resetTraces := flags.Bool("reset-traces", false, "")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		invalidInput()
	}
	root, err := os.Getwd()
	if err != nil {
		return fail(&loadtest.Failure{Code: 4, Reason: "environment_unavailable"})
	}
	if err := (loadtest.Environment{Root: root}).Reset(context.Background(), *confirm, *resetTraces); err != nil {
		return fail(err)
	}
	return 0
}
