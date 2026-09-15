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
	case "lint":
		return runLint(args[1:], stdout, stderr)
	case "diff":
		return runDiff(args[1:], stdout, stderr)
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
		stages       []Stage
	)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			showHelpFlag = true
		case "--stage", "-s":
			i++
			if i >= len(args) {
				_, _ = fmt.Fprintln(stderr, "missing value for stage")
				return 2
			}
			for _, raw := range strings.Split(args[i], ",") {
				st, ok := ParseStage(strings.TrimSpace(raw))
				if !ok {
					_, _ = fmt.Fprintf(stderr, "unknown stage %q (want current, staging or future)\n", raw)
					return 2
				}
				stages = append(stages, st)
			}
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
	if validationErrors := Validate(model); len(validationErrors) > 0 {
		_, _ = fmt.Fprintf(stderr, "invalid model: %v\n", validationErrors[0])
		return 1
	}
	if len(stages) > 0 {
		model = FilterStages(model, stages...)
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

// runLint parses and validates a file, then prints every advisory finding
// (open hotspots, uncovered commands, ...). With --strict it exits 1 when
// any finding remains, so "all open questions resolved" can gate a merge.
func runLint(args []string, stdout, stderr io.Writer) int {
	var (
		inputPath string
		strict    bool
	)
	for _, a := range args {
		switch a {
		case "-h", "--help":
			printUsage(stdout)
			return 0
		case "--strict":
			strict = true
		default:
			if strings.HasPrefix(a, "-") {
				_, _ = fmt.Fprintf(stderr, "unknown flag %q\n", a)
				return 2
			}
			if inputPath != "" {
				_, _ = fmt.Fprintln(stderr, "lint requires exactly one input file")
				return 2
			}
			inputPath = a
		}
	}
	if inputPath == "" {
		_, _ = fmt.Fprintln(stderr, "lint requires exactly one input file")
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
	exit := 0
	if errs := Validate(model); len(errs) > 0 {
		for _, e := range errs {
			_, _ = fmt.Fprintf(stderr, "error: %v\n", e)
		}
		exit = 1
	}
	findings := Lint(model)
	for _, f := range findings {
		_, _ = fmt.Fprintln(stdout, f.String())
	}
	hotspots := 0
	for _, f := range findings {
		if f.Kind == "hotspot" {
			hotspots++
		}
	}
	_, _ = fmt.Fprintf(stdout, "%d frame(s), %d scenario(s), %d open question(s), %d finding(s)\n", len(model.Frames), len(model.GWTs), hotspots, len(findings))
	if strict && len(findings) > 0 {
		return 1
	}
	return exit
}

// runDiff prints a semantic comparison of two versions of a flow.
func runDiff(args []string, stdout, stderr io.Writer) int {
	var paths []string
	for _, a := range args {
		switch a {
		case "-h", "--help":
			printUsage(stdout)
			return 0
		default:
			if strings.HasPrefix(a, "-") {
				_, _ = fmt.Fprintf(stderr, "unknown flag %q\n", a)
				return 2
			}
			paths = append(paths, a)
		}
	}
	if len(paths) != 2 {
		_, _ = fmt.Fprintln(stderr, "diff requires exactly two input files: <before> <after>")
		return 2
	}
	models := make([]*Model, 2)
	for i, p := range paths {
		content, err := readFile(p)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "read input: %v\n", err)
			return 1
		}
		m, err := Parse(string(content))
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "parse %s: %v\n", p, err)
			return 1
		}
		models[i] = m
	}
	d := Diff(models[0], models[1])
	if d.Empty() {
		_, _ = fmt.Fprintln(stdout, "no differences")
		return 0
	}
	for _, line := range d.Summary() {
		_, _ = fmt.Fprintln(stdout, line)
	}
	return 0
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  evml svg <file> [-d <dir>] [-o <file>] [--stage current[,staging[,future]]]")
	_, _ = fmt.Fprintln(w, "  evml lint <file> [--strict]")
	_, _ = fmt.Fprintln(w, "  evml diff <before.evml> <after.evml>")
	_, _ = fmt.Fprintln(w, "  evml --version")
}
