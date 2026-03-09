package main

import (
	"fmt"
	"strings"
)

// Error kinds
type ErrorKind string

const (
	ERR_SYNTAX    ErrorKind = "SyntaxError"
	ERR_TYPE      ErrorKind = "TypeError"
	ERR_RUNTIME   ErrorKind = "RuntimeError"
	ERR_NETWORK   ErrorKind = "NetworkError"
	ERR_UNDEFINED ErrorKind = "UndefinedError"
	ERR_IO        ErrorKind = "IOError"
)

// ANSI colors
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorGreen  = "\033[32m"
	colorMagenta = "\033[35m"
	underline   = "\033[4m"
)

// VexError is the main error type with rich display info
type VexError struct {
	Kind    ErrorKind
	Message string
	File    string
	Line    int
	Col     int
	Lines   []string
	Hint    string
	Fix     string
	Notes   []string
}

func NewVexError(kind ErrorKind, msg, file string, line, col int, lines []string, hint, fix string, notes ...string) *VexError {
	return &VexError{
		Kind:    kind,
		Message: msg,
		File:    file,
		Line:    line,
		Col:     col,
		Lines:   lines,
		Hint:    hint,
		Fix:     fix,
		Notes:   notes,
	}
}

func (e *VexError) Error() string {
	return e.Render()
}

func (e *VexError) Render() string {
	var sb strings.Builder

	// Header: error[Kind]: message
	sb.WriteString(fmt.Sprintf("%s%serror[%s]%s%s: %s%s\n",
		colorBold, colorRed, e.Kind, colorReset, colorBold, e.Message, colorReset))

	// Location: --> file:line:col
	sb.WriteString(fmt.Sprintf("  %s-->%s %s%s:%d:%d%s\n",
		colorBlue, colorReset, colorDim, e.File, e.Line, e.Col, colorReset))

	// Gutter separator
	lineNumWidth := len(fmt.Sprintf("%d", e.Line+1))
	gutter := strings.Repeat(" ", lineNumWidth)

	sb.WriteString(fmt.Sprintf("  %s%s |%s\n", colorBlue, gutter, colorReset))

	// Source lines (show line before, target line, line after)
	for i := e.Line - 1; i <= e.Line+1; i++ {
		if i < 1 || i > len(e.Lines) {
			continue
		}
		srcLine := e.Lines[i-1]

		if i == e.Line {
			// Highlighted line
			sb.WriteString(fmt.Sprintf("  %s%*d |%s %s\n",
				colorBlue, lineNumWidth, i, colorReset, srcLine))

			// Underline with carets
			colIdx := e.Col - 1
			if colIdx < 0 {
				colIdx = 0
			}
			// Determine token length
			tokenLen := 1
			if colIdx < len(srcLine) {
				// Try to find word boundary
				end := colIdx
				for end < len(srcLine) && srcLine[end] != ' ' && srcLine[end] != '(' && srcLine[end] != ')' {
					end++
				}
				tokenLen = end - colIdx
				if tokenLen < 1 {
					tokenLen = 1
				}
			}

			prefix := strings.Repeat(" ", colIdx)
			carets := strings.Repeat("^", tokenLen)
			sb.WriteString(fmt.Sprintf("  %s%s |%s %s%s%s%s",
				colorBlue, gutter, colorReset,
				prefix, colorRed+colorBold, carets, colorReset))

			if e.Hint != "" {
				sb.WriteString(fmt.Sprintf(" %s%s%s", colorYellow, e.Hint, colorReset))
			}
			sb.WriteString("\n")
		} else {
			sb.WriteString(fmt.Sprintf("  %s%*d |%s %s%s%s\n",
				colorBlue, lineNumWidth, i, colorReset, colorDim, srcLine, colorReset))
		}
	}

	sb.WriteString(fmt.Sprintf("  %s%s |%s\n", colorBlue, gutter, colorReset))

	// Fix suggestion
	if e.Fix != "" {
		sb.WriteString(fmt.Sprintf("\n  %s💡 help:%s %s\n", colorGreen+colorBold, colorReset, e.Fix))
	}

	// Notes
	for _, note := range e.Notes {
		sb.WriteString(fmt.Sprintf("  %s📝 note:%s %s\n", colorCyan+colorBold, colorReset, note))
	}

	return sb.String()
}

// ErrorReporter collects and renders multiple errors
type ErrorReporter struct {
	errors []*VexError
	file   string
	lines  []string
}

func NewErrorReporter(file string, src string) *ErrorReporter {
	return &ErrorReporter{
		file:  file,
		lines: strings.Split(src, "\n"),
	}
}

func (r *ErrorReporter) Add(e *VexError) {
	r.errors = append(r.errors, e)
}

func (r *ErrorReporter) HasErrors() bool {
	return len(r.errors) > 0
}

func (r *ErrorReporter) Count() int {
	return len(r.errors)
}

func (r *ErrorReporter) Render() string {
	var sb strings.Builder
	for _, e := range r.errors {
		sb.WriteString(e.Render())
		sb.WriteString("\n")
	}
	if len(r.errors) > 0 {
		sb.WriteString(fmt.Sprintf("%s%saborting due to %d error(s)%s\n",
			colorBold, colorRed, len(r.errors), colorReset))
	}
	return sb.String()
}

// RuntimeError creates a quick runtime error
func RuntimeError(msg, file string, line, col int, lines []string) *VexError {
	return &VexError{
		Kind:    ERR_RUNTIME,
		Message: msg,
		File:    file,
		Line:    line,
		Col:     col,
		Lines:   lines,
	}
}

// NetworkError creates a network-specific error
func NetworkError(msg, file string, line, col int, lines []string, hint, fix string) *VexError {
	return NewVexError(ERR_NETWORK, msg, file, line, col, lines, hint, fix)
}
