package main

import (
	"bufio"
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

%sLEARN MORE:%s
  https://github.com/vex-lang/vex
`, colorBold+colorCyan, colorReset, VERSION,
		colorBold+colorYellow, colorReset,
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

// ===== REPL =====

func runREPL() {
	fmt.Printf("%s%s%s", colorCyan, banner, colorReset)
	fmt.Printf("%svex %s%s — network scripting language\n", colorBold, VERSION, colorReset)
	fmt.Printf("%stype 'exit' or Ctrl+C to quit, 'help' for help%s\n\n", colorDim, colorReset)

	interp := NewInterpreter(nil)
	scanner := bufio.NewScanner(os.Stdin)

	var history []string
	_ = history

	for {
		fmt.Printf("%s›%s ", colorCyan+colorBold, colorReset)
		if !scanner.Scan() {
			fmt.Println()
			break
		}

		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}
		if trimmed == "exit" || trimmed == "quit" {
			fmt.Printf("%sBye! 👋%s\n", colorCyan, colorReset)
			break
		}
		if trimmed == "help" {
			printReplHelp()
			continue
		}
		if trimmed == "clear" {
			fmt.Print("\033[H\033[2J")
			continue
		}

		// Multi-line detection: if line ends with { or (, collect more
		src := line
		for isIncomplete(src) {
			fmt.Printf("%s·%s ", colorDim, colorReset)
			if !scanner.Scan() { break }
			src += "\n" + scanner.Text()
		}

		history = append(history, src)
		evalREPL(interp, src)
	}
}

func isIncomplete(src string) bool {
	open := 0
	inStr := false
	strChar := rune(0)
	runes := []rune(src)
	for _, ch := range runes {
		if inStr {
			if ch == strChar { inStr = false }
		} else if ch == '"' || ch == '\'' || ch == '`' {
			inStr = true; strChar = ch
		} else if ch == '{' || ch == '(' || ch == '[' {
			open++
		} else if ch == '}' || ch == ')' || ch == ']' {
			open--
		}
	}
	return open > 0
}

func evalREPL(interp *Interpreter, src string) {
	lexer := NewLexer(src, "<repl>")
	tokens, lexErrs := lexer.Tokenize()

	for _, e := range lexErrs {
		fmt.Print(e.Render())
	}
	if len(lexErrs) > 0 {
		return
	}

	parser := NewParser(tokens, "<repl>", src)
	prog, parseErrs := parser.Parse()

	for _, e := range parseErrs {
		fmt.Print(e.Render())
	}
	if len(parseErrs) > 0 {
		return
	}

	// For REPL: if last statement is an expression, print the result
	if len(prog.Stmts) > 0 {
		last := prog.Stmts[len(prog.Stmts)-1]
		if exprStmt, isExpr := last.(*ExprStmt); isExpr {
			// Execute all but last
			for _, stmt := range prog.Stmts[:len(prog.Stmts)-1] {
				res := interp.execStmt(stmt, interp.globals)
				if res.Err != nil {
					if vexErr, ok := res.Err.(*VexError); ok {
						fmt.Print(vexErr.Render())
					} else {
						fmt.Fprintf(os.Stderr, "%serror:%s %v\n", colorRed, colorReset, res.Err)
					}
					return
				}
			}
			// Eval and print last expr
			interp.file = "<repl>"
			interp.lines = strings.Split(src, "\n")
			res := interp.evalExpr(exprStmt.Expr, interp.globals)
			if res.Err != nil {
				if vexErr, ok := res.Err.(*VexError); ok {
					fmt.Print(vexErr.Render())
				} else {
					fmt.Fprintf(os.Stderr, "%serror:%s %v\n", colorRed, colorReset, res.Err)
				}
				return
			}
			if res.Value != nil && res.Value.Type != VAL_NULL {
				fmt.Printf("%s=%s %s%s%s\n", colorDim, colorReset, colorGreen, res.Value.Repr(), colorReset)
			}
			return
		}
	}

	interp.file = "<repl>"
	interp.lines = strings.Split(src, "\n")
	res := interp.execBlock(&BlockStmt{Stmts: prog.Stmts}, interp.globals)
	if res.Err != nil {
		if vexErr, ok := res.Err.(*VexError); ok {
			fmt.Print(vexErr.Render())
		} else {
			fmt.Fprintf(os.Stderr, "%serror:%s %v\n", colorRed, colorReset, res.Err)
		}
	}
}

func printReplHelp() {
	fmt.Printf(`
%sVex REPL Commands:%s
  exit / quit    Exit the REPL
  clear          Clear the screen
  help           Show this help

%sQuick Examples:%s
  let x = 42
  print("hello", x)
  let res = fetch "https://httpbin.org/get"
  res.status
  let data = res.json()
  [1,2,3].map(fn(x) { x * 2 })
  spawn some_fn()            ← run concurrently

%sNetwork:%s
  fetch "https://..."        HTTP GET
  fetch "url" { method: "POST", body: data }
  connect "localhost:9000"   TCP connect  
  listen "0.0.0.0:9000"      TCP listen
  serve "0.0.0.0:8080" { GET "/" => fn(req, res) { res.send("hi") } }

`, colorBold+colorCyan, colorReset,
		colorBold+colorYellow, colorReset,
		colorBold+colorYellow, colorReset)
}
