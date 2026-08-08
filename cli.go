package evml

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const Version = "0.1.0"

var (
	readFile   = os.ReadFile
	writeFile  = os.WriteFile
	makeDirAll = os.MkdirAll
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "--help", "-h", "help":
		printUsage(stdout)
		return 0
	case "--version", "version":
		_, _ = fmt.Fprintln(stdout, Version)
		return 0
	case "svg":
		return runSVG(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runSVG(args []string, stdout, stderr io.Writer) int {
	var (
		inputPath    string
		destination  string
		output       string
		showHelpFlag bool
	)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			showHelpFlag = true
		case "-d", "--destination":
			i++
			if i >= len(args) {
				_, _ = fmt.Fprintln(stderr, "missing value for destination")
				return 2
			}
			destination = args[i]
		case "-o", "--output":
			i++
			if i >= len(args) {
				_, _ = fmt.Fprintln(stderr, "missing value for output")
				return 2
			}
			output = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				_, _ = fmt.Fprintf(stderr, "unknown flag %q\n", args[i])
				return 2
			}
			if inputPath != "" {
				_, _ = fmt.Fprintln(stderr, "svg requires exactly one input file")
				return 2
			}
			inputPath = args[i]
		}
	}
	if showHelpFlag {
		printUsage(stdout)
		return 0
	}
	if inputPath == "" {
		_, _ = fmt.Fprintln(stderr, "svg requires exactly one input file")
		return 2
	}
	content, err := readFile(inputPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "read input: %v\n", err)
		return 1
	}
	model, err := Parse(string(content))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "parse evml: %v\n", err)
		return 1
	}
	if validationErrors := ValidateConnections(model); len(validationErrors) > 0 {
		_, _ = fmt.Fprintf(stderr, "invalid model: %v\n", validationErrors[0])
		return 1
	}
	svg, err := RenderSVG(model, RenderOptions{})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "render svg: %v\n", err)
		return 1
	}
	target := output
	if target == "" {
		target = defaultOutputPath(inputPath, destination)
	}
	if err := makeDirAll(filepath.Dir(target), 0o755); err != nil {
		_, _ = fmt.Fprintf(stderr, "create output directory: %v\n", err)
		return 1
	}
	if err := writeFile(target, []byte(svg), 0o644); err != nil {
		_, _ = fmt.Fprintf(stderr, "write output: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "SVG generated successfully: %s\n", target)
	return 0
}

func defaultOutputPath(inputPath, destination string) string {
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath)) + ".svg"
	if destination != "" {
		return filepath.Join(destination, base)
	}
	return filepath.Join(filepath.Dir(inputPath), "generated", base)
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  evml svg <file> [-d <dir>]")
	_, _ = fmt.Fprintln(w, "  evml --version")
}
