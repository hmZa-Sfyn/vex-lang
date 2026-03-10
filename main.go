package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const VERSION = "0.1.0"

const banner = `
 ██╗   ██╗███████╗██╗  ██╗      ██╗      █████╗ ███╗   ██╗ ██████╗ 
 ██║   ██║██╔════╝╚██╗██╔╝      ██║     ██╔══██╗████╗  ██║██╔════╝ 
 ██║   ██║█████╗   ╚███╔╝ █████╗██║     ███████║██╔██╗ ██║██║  ███╗
 ╚██╗ ██╔╝██╔══╝   ██╔██╗ ╚════╝██║     ██╔══██║██║╚██╗██║██║   ██║
  ╚████╔╝ ███████╗██╔╝ ██╗      ███████╗██║  ██║██║ ╚████║╚██████╔╝
   ╚═══╝  ╚══════╝╚═╝  ╚═╝      ╚══════╝╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝ 
`

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		runREPL()
		return
	}

	switch args[0] {
	case "--version", "-v":
		fmt.Printf("vex %s\n", VERSION)
	case "--help", "-h":
		printHelp()
	case "run":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: vex run <file.vex>")
			os.Exit(1)
		}
		runFile(args[1], args[2:])
	case "repl":
		runREPL()
	case "check":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: vex check <file.vex>")
			os.Exit(1)
		}
		checkFile(args[1])
	default:
		// Treat as file path
		runFile(args[0], args[1:])
	}
}

func printHelp() {
	fmt.Printf(`%svex%s - network-first scripting language v%s

%sUSAGE:%s
  vex [command] [options]

%sCOMMANDS:%s
  vex <file.vex>      Run a vex script
  vex run <file.vex>  Run a vex script  
  vex repl            Start interactive REPL
  vex check <file>    Check syntax without running
  vex --version       Print version

%sEXAMPLES:%s
  vex server.vex
  vex run client.vex
  vex repl

`, colorBold+colorCyan, colorReset, VERSION,
		colorBold+colorYellow, colorReset,
		colorBold+colorYellow, colorReset,
		colorBold+colorYellow, colorReset)
}

func runFile(path string, extraArgs []string) {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%serror:%s cannot open '%s': %v\n", colorRed+colorBold, colorReset, path, err)
		os.Exit(1)
	}

	file := filepath.Base(path)
	runSource(string(src), file)
}

func checkFile(path string) {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%serror:%s cannot open '%s': %v\n", colorRed+colorBold, colorReset, path, err)
		os.Exit(1)
	}

	file := filepath.Base(path)
	srcStr := string(src)

	lexer := NewLexer(srcStr, file)
	tokens, lexErrs := lexer.Tokenize()

	reporter := NewErrorReporter(file, srcStr)
	for _, e := range lexErrs {
		reporter.Add(e)
	}

	parser := NewParser(tokens, file, srcStr)
	_, parseErrs := parser.Parse()
	for _, e := range parseErrs {
		reporter.Add(e)
	}

	if reporter.HasErrors() {
		fmt.Print(reporter.Render())
		os.Exit(1)
	}

	fmt.Printf("%s✓ %s — no errors found%s\n", colorGreen+colorBold, file, colorReset)
}

func runSource(src, file string) {
	lexer := NewLexer(src, file)
	tokens, lexErrs := lexer.Tokenize()

	reporter := NewErrorReporter(file, src)
	for _, e := range lexErrs {
		reporter.Add(e)
	}

	if reporter.HasErrors() {
		fmt.Print(reporter.Render())
		os.Exit(1)
	}

	parser := NewParser(tokens, file, src)
	prog, parseErrs := parser.Parse()

	for _, e := range parseErrs {
		reporter.Add(e)
	}

	if reporter.HasErrors() {
		fmt.Print(reporter.Render())
		os.Exit(1)
	}

	interp := NewInterpreter(nil)
	if err := interp.Run(prog); err != nil {
		if vexErr, ok := err.(*VexError); ok {
			fmt.Print(vexErr.Render())
		} else {
			// Build a synthetic runtime error
			vErr := &VexError{
				Kind:    ERR_RUNTIME,
				Message: err.Error(),
				File:    file,
				Line:    1,
				Col:     1,
				Lines:   strings.Split(src, "\n"),
			}
			fmt.Print(vErr.Render())
		}
		os.Exit(1)
	}
}



