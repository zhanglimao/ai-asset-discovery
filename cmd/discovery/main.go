package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ai-asset-discovery/internal/discovery"
)

func main() {
	rulesPath := flag.String("rules", "rules", "Path to rules file or directory")
	outputPath := flag.String("output", "", "Output JSON file (default: stdout)")
	pretty := flag.Bool("pretty", true, "Pretty-print JSON output")
	flag.Parse()

	engine := discovery.NewEngine()

	if err := engine.LoadRules(*rulesPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading rules: %v\n", err)
		os.Exit(1)
	}

	result, err := engine.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running discovery: %v\n", err)
		os.Exit(1)
	}

	var output []byte
	if *pretty {
		output, err = json.MarshalIndent(result, "", "  ")
	} else {
		output, err = json.Marshal(result)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling result: %v\n", err)
		os.Exit(1)
	}

	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, output, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Results written to %s\n", *outputPath)
	} else {
		fmt.Println(string(output))
	}
}
