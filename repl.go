package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unicode/utf8"
	"unsafe"
)

// ══════════════════════════════════════════════════════════════════════════════
//  RAW TERMINAL  (zero external deps — pure Linux syscall)
// ══════════════════════════════════════════════════════════════════════════════

type rawTermios struct{ Iflag, Oflag, Cflag, Lflag uint32; Line uint8; Cc [32]uint8; pad [3]byte }

var savedTermios rawTermios

func termRaw(fd int) error {
	var t rawTermios
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		syscall.TCGETS, uintptr(unsafe.Pointer(&t))); e != 0 {
		return e
	}
	savedTermios = t
	t.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP |
		syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	t.Oflag &^= syscall.OPOST
	t.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	t.Cflag &^= syscall.CSIZE | syscall.PARENB
	t.Cflag |= syscall.CS8
	t.Cc[syscall.VMIN] = 1
	t.Cc[syscall.VTIME] = 0
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		syscall.TCSETS, uintptr(unsafe.Pointer(&t))); e != 0 {
		return e
	}
	return nil
}

func termRestore(fd int) {
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		syscall.TCSETS, uintptr(unsafe.Pointer(&savedTermios)))
}

func termSize() (w, h int) {
	type ws struct{ Row, Col, X, Y uint16 }
	var s ws
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(os.Stdout.Fd()),
		syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&s)))
	w, h = int(s.Col), int(s.Row)
	if w < 20 { w = 80 }
	if h < 4 { h = 24 }
	return
}

// ══════════════════════════════════════════════════════════════════════════════
//  KEY INPUT
// ══════════════════════════════════════════════════════════════════════════════

type key uint16

const (
	kRune     key = iota
	kEnter        // CR / LF  → submit
	kNewline       // Ctrl+J or Alt+Enter → insert literal \n
	kBack         // backspace
	kDel          // Delete key
	kLeft         // ← arrow
	kRight        // → arrow
	kUp           // ↑ arrow  (history back)
	kDown         // ↓ arrow  (history forward)
	kHome         // Home / Ctrl+A
	kEnd          // End  / Ctrl+E
	kWordL        // Ctrl+← / Alt+b  (word left)
	kWordR        // Ctrl+→ / Alt+f  (word right)
	kKillEOL      // Ctrl+K
	kKillLine     // Ctrl+U
	kKillWordB    // Ctrl+W
	kCtrlC
	kCtrlD
	kCtrlL        // clear screen
	kTab
	kUnknown
)

type keyEvent struct {
	k key
	r rune
}

// readKey reads one keypress from stdin (raw mode assumed)
func readKey() keyEvent {
	b := make([]byte, 1)
	if n, _ := os.Stdin.Read(b); n == 0 {
		return keyEvent{k: kCtrlD}
	}
	c := b[0]

	// ── multi-byte UTF-8 rune ──
	if c >= 0x80 {
		sz := 0
		switch {
		case c&0xE0 == 0xC0: sz = 2
		case c&0xF0 == 0xE0: sz = 3
		case c&0xF8 == 0xF0: sz = 4
		default: return keyEvent{k: kUnknown}
		}
		rest := make([]byte, sz-1)
		os.Stdin.Read(rest)
		r, _ := utf8.DecodeRune(append([]byte{c}, rest...))
		return keyEvent{k: kRune, r: r}
	}

	// ── control characters ──
	switch c {
	case 0x01: return keyEvent{k: kHome}        // Ctrl+A
	case 0x02: return keyEvent{k: kLeft}        // Ctrl+B
	case 0x03: return keyEvent{k: kCtrlC}
	case 0x04: return keyEvent{k: kCtrlD}
	case 0x05: return keyEvent{k: kEnd}         // Ctrl+E
	case 0x06: return keyEvent{k: kRight}       // Ctrl+F
	case 0x0A: return keyEvent{k: kNewline}     // Ctrl+J  → literal newline
	case 0x0B: return keyEvent{k: kKillEOL}     // Ctrl+K
	case 0x0C: return keyEvent{k: kCtrlL}
	case 0x0D: return keyEvent{k: kEnter}       // CR
	case 0x0E: return keyEvent{k: kDown}        // Ctrl+N
	case 0x10: return keyEvent{k: kUp}          // Ctrl+P
	case 0x15: return keyEvent{k: kKillLine}    // Ctrl+U
	case 0x17: return keyEvent{k: kKillWordB}   // Ctrl+W
	case 0x09: return keyEvent{k: kTab}
	case 0x7F, 0x08: return keyEvent{k: kBack}

	case 0x1B: // ESC sequence
		nb := make([]byte, 1)
		n, _ := os.Stdin.Read(nb)
		if n == 0 { return keyEvent{k: kUnknown} }

		if nb[0] == '[' || nb[0] == 'O' {
			// CSI sequence
			nb2 := make([]byte, 1)
			os.Stdin.Read(nb2)
			switch nb2[0] {
			case 'A': return keyEvent{k: kUp}
			case 'B': return keyEvent{k: kDown}
			case 'C': return keyEvent{k: kRight}
			case 'D': return keyEvent{k: kLeft}
			case 'H': return keyEvent{k: kHome}
			case 'F': return keyEvent{k: kEnd}
			case '1', '7':
				nb3 := make([]byte, 2)
				os.Stdin.Read(nb3)
				if nb3[0] == ';' {
					os.Stdin.Read(nb3[1:])
					mod := nb3[1]
					dir := make([]byte, 1)
					os.Stdin.Read(dir)
					if mod == '5' || mod == '3' { // Ctrl or Alt
						switch dir[0] {
						case 'C': return keyEvent{k: kWordR}
						case 'D': return keyEvent{k: kWordL}
						}
					}
				} else if nb3[0] == '~' {
					return keyEvent{k: kHome}
				}
			case '3':
				nb3 := make([]byte, 1)
				os.Stdin.Read(nb3)
				if nb3[0] == '~' { return keyEvent{k: kDel} }
			case '4', '8':
				nb3 := make([]byte, 1)
				os.Stdin.Read(nb3)
				if nb3[0] == '~' { return keyEvent{k: kEnd} }
			case '5', '6':
				nb3 := make([]byte, 1)
				os.Stdin.Read(nb3) // consume ~
			}
			return keyEvent{k: kUnknown}
		}

		// Alt+key
		switch nb[0] {
		case 0x0D: return keyEvent{k: kNewline}  // Alt+Enter
		case 'b':  return keyEvent{k: kWordL}
		case 'f':  return keyEvent{k: kWordR}
		case 'd':  return keyEvent{k: kKillEOL}
		}
		return keyEvent{k: kUnknown}
	}

	// printable ASCII
	if c >= 0x20 && c < 0x7F {
		return keyEvent{k: kRune, r: rune(c)}
	}
	return keyEvent{k: kUnknown}
}

// ══════════════════════════════════════════════════════════════════════════════
//  LINE BUFFER  (rune-based, supports embedded newlines for multi-line edits)
// ══════════════════════════════════════════════════════════════════════════════

type lineBuf struct {
	rs  []rune
	cur int // cursor position in rune slice
}

func (b *lineBuf) String() string        { return string(b.rs) }
func (b *lineBuf) Len() int              { return len(b.rs) }
func (b *lineBuf) set(s string)          { b.rs = []rune(s); b.cur = len(b.rs) }
func (b *lineBuf) reset()                { b.rs = b.rs[:0]; b.cur = 0 }

func (b *lineBuf) insertRune(r rune) {
	b.rs = append(b.rs[:b.cur], append([]rune{r}, b.rs[b.cur:]...)...)
	b.cur++
}

func (b *lineBuf) backspace() {
	if b.cur == 0 { return }
	b.rs = append(b.rs[:b.cur-1], b.rs[b.cur:]...)
	b.cur--
}

func (b *lineBuf) deleteForward() {
	if b.cur >= len(b.rs) { return }
	b.rs = append(b.rs[:b.cur], b.rs[b.cur+1:]...)
}

func (b *lineBuf) moveLeft()  { if b.cur > 0 { b.cur-- } }
func (b *lineBuf) moveRight() { if b.cur < len(b.rs) { b.cur++ } }

func (b *lineBuf) moveWordLeft() {
	for b.cur > 0 && isWordSep(b.rs[b.cur-1]) { b.cur-- }
	for b.cur > 0 && !isWordSep(b.rs[b.cur-1]) { b.cur-- }
}

func (b *lineBuf) moveWordRight() {
	for b.cur < len(b.rs) && isWordSep(b.rs[b.cur]) { b.cur++ }
	for b.cur < len(b.rs) && !isWordSep(b.rs[b.cur]) { b.cur++ }
}

func isWordSep(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '.' || r == '(' || r == ')' || r == '{' || r == '}'
}

// moveToLineStart moves cursor to start of the logical line the cursor is on
func (b *lineBuf) moveToLineStart() {
	i := b.cur - 1
	for i >= 0 && b.rs[i] != '\n' { i-- }
	b.cur = i + 1
}

// moveToLineEnd moves cursor to end of current logical line
func (b *lineBuf) moveToLineEnd() {
	for b.cur < len(b.rs) && b.rs[b.cur] != '\n' { b.cur++ }
}

// moveLineUp moves the cursor to the same column on the line above
func (b *lineBuf) moveLineUp() {
	// find current line start + col
	ls := b.lineStart(b.cur)
	col := b.cur - ls
	if ls == 0 { b.cur = 0; return }
	prevEnd := ls - 1 // the \n before current line
	prevStart := b.lineStart(prevEnd)
	prevLen := prevEnd - prevStart
	if col > prevLen { col = prevLen }
	b.cur = prevStart + col
}

// moveLineDown moves the cursor to the same column on the line below
func (b *lineBuf) moveLineDown() {
	ls := b.lineStart(b.cur)
	col := b.cur - ls
	// find end of current line
	end := b.cur
	for end < len(b.rs) && b.rs[end] != '\n' { end++ }
	if end >= len(b.rs) { b.cur = len(b.rs); return }
	nextStart := end + 1
	nextEnd := nextStart
	for nextEnd < len(b.rs) && b.rs[nextEnd] != '\n' { nextEnd++ }
	nextLen := nextEnd - nextStart
	if col > nextLen { col = nextLen }
	b.cur = nextStart + col
}

func (b *lineBuf) lineStart(pos int) int {
	i := pos - 1
	for i >= 0 && b.rs[i] != '\n' { i-- }
	return i + 1
}

// killToEOL deletes from cursor to end of current logical line
func (b *lineBuf) killToEOL() {
	end := b.cur
	for end < len(b.rs) && b.rs[end] != '\n' { end++ }
	b.rs = append(b.rs[:b.cur], b.rs[end:]...)
}

// killWordBack deletes the word to the left of the cursor
func (b *lineBuf) killWordBack() {
	old := b.cur
	b.moveWordLeft()
	b.rs = append(b.rs[:b.cur], b.rs[old:]...)
}

// splitLines returns the buffer content as lines, and the cursor's (line, col)
func (b *lineBuf) splitLines() (lines []string, curLine, curCol int) {
	all := b.String()
	lines = strings.Split(all, "\n")
	if len(lines) == 0 { lines = []string{""} }

	pos := 0
	for li, l := range lines {
		end := pos + len([]rune(l))
		if b.cur <= end {
			curLine = li
			curCol = b.cur - pos
			return
		}
		pos = end + 1 // +1 for the \n
	}
	// cursor at very end
	curLine = len(lines) - 1
	curCol = len([]rune(lines[curLine]))
	return
}

// ══════════════════════════════════════════════════════════════════════════════
//  HISTORY  (up to 500 entries, no-duplicate adjacent, Ctrl+P/N navigation)
// ══════════════════════════════════════════════════════════════════════════════

const histMax = 500

type history struct {
	items []string
	pos   int    // browse position; == len(items) means "current (unsaved) input"
	draft string // saved draft when user starts browsing
}

func (h *history) push(s string) {
	s = strings.TrimSpace(s)
	if s == "" { return }
	if len(h.items) > 0 && h.items[len(h.items)-1] == s { goto done }
	h.items = append(h.items, s)
	if len(h.items) > histMax {
		h.items = h.items[1:]
	}
done:
	h.pos = len(h.items)
}

// prev moves one step back in history; returns the entry and true if moved
func (h *history) prev(cur string) (string, bool) {
	if len(h.items) == 0 { return cur, false }
	if h.pos == len(h.items) { h.draft = cur } // save draft before first move
	if h.pos > 0 { h.pos-- }
	return h.items[h.pos], true
}

// next moves one step forward; returns draft when past the end
func (h *history) next() (string, bool) {
	if h.pos >= len(h.items) { return h.draft, false }
	h.pos++
	if h.pos == len(h.items) { return h.draft, true }
	return h.items[h.pos], true
}

func (h *history) resetPos() { h.pos = len(h.items) }

// ══════════════════════════════════════════════════════════════════════════════
//  RENDERER  — redraws the prompt area in-place after every keystroke
// ══════════════════════════════════════════════════════════════════════════════

const (
	ansiClearLine  = "\033[2K"  // erase entire line
	ansiClearRight = "\033[K"   // erase to end of line
)

func ansiUp(n int) string  { return fmt.Sprintf("\033[%dA", n) }
func ansiCol(c int) string { return fmt.Sprintf("\033[%dG", c+1) } // 1-based

const (
	promptMain = "❯ "  // 2 visible runes
	promptCont = "· "  // continuation line, 2 visible runes
	promptVis  = 2     // visible width of prompts above
)

type renderer struct {
	prevRows int // how many terminal rows we painted last time
}

// paint redraws everything.  Must be called with cursor at col-0 of top row.
func (r *renderer) paint(b *lineBuf, hint string) {
	w, _ := termSize()
	lines, curLine, curCol := b.splitLines()

	// ── move up to erase previous paint ──
	if r.prevRows > 0 {
		fmt.Print(ansiUp(r.prevRows))
	}
	fmt.Print("\r")

	// ── draw each logical line ──
	totalRows := 0
	for i, l := range lines {
		fmt.Print(ansiClearLine + "\r")
		if i == 0 {
			fmt.Print(colorCyan + colorBold + promptMain + colorReset)
		} else {
			fmt.Print(colorDim + promptCont + colorReset)
		}
		fmt.Print(l)

		// hint only on last line, after content
		if i == len(lines)-1 && hint != "" {
			fmt.Print(colorDim + "  " + hint + colorReset)
		}
		fmt.Print(ansiClearRight)

		// count terminal rows this logical line occupies
		visLen := promptVis + len([]rune(l))
		rows := 1 + visLen/w
		totalRows += rows

		if i < len(lines)-1 {
			fmt.Print("\r\n")
		}
	}
	r.prevRows = totalRows - 1

	// ── reposition cursor ──
	// We are currently at the last drawn row. Move up to curLine's row.
	rowsFromBottom := len(lines) - 1 - curLine
	// also account for line wraps above curLine
	for i := curLine + 1; i < len(lines); i++ {
		visLen := promptVis + len([]rune(lines[i]))
		rowsFromBottom += visLen / w
	}
	if rowsFromBottom > 0 {
		fmt.Print(ansiUp(rowsFromBottom))
	}
	// horizontal position
	termCol := (promptVis + curCol) % w
	fmt.Print(ansiCol(termCol))
}

// commit moves the terminal cursor past the rendered block so output appears below it.
func (r *renderer) commit() {
	if r.prevRows > 0 {
		fmt.Printf("\033[%dB", r.prevRows) // move down to last row
	}
	fmt.Print("\r\n")
	r.prevRows = 0
}

// ══════════════════════════════════════════════════════════════════════════════
//  INLINE HINTS  (ghost text after cursor)
// ══════════════════════════════════════════════════════════════════════════════

var snippets = map[string]string{
	"fn":      "fn name(args) { }",
	"if":      "if cond { }",
	"else":    "else { }",
	"for":     "for x in list { }",
	"while":   "while cond { }",
	"loop":    "loop { }",
	"match":   "match val { x => { } _ => { } }",
	"struct":  "struct Name { field = default }",
	"enum":    "enum Name { A = 0, B = 1 }",
	"impl":    "impl Name { fn method(self) { } }",
	"try":     "try { } catch err { }",
	"fetch":   `fetch "https://..."`,
	"serve":   `serve "0.0.0.0:8080" { GET "/" => fn(req,res){ } }`,
	"spawn":   "spawn fn()",
	"bg":      "bg { }",
	"let":     "let name = value",
	"const":   "const NAME = value",
	"return":  "return value",
	"throw":   "throw \"message\"",
	"new":     "new TypeName { field: val }",
	"connect": `connect "host:port"`,
	"listen":  `listen "0.0.0.0:9000"`,
}

var kwList = []string{
	"let", "const", "fn", "return", "if", "else", "while", "for", "in", "loop",
	"match", "struct", "enum", "impl", "new", "try", "catch", "finally", "throw",
	"do", "unless", "until", "defer", "spawn", "await", "fetch", "serve",
	"connect", "listen", "send", "recv", "bg", "break", "continue",
	"print", "println", "json", "math", "time", "os", "process",
	"regex", "emitter", "compose", "partial", "memoize", "sh", "exec",
}

// hint returns ghost text for the word the cursor is sitting after
func hint(curLineText string, curCol int) string {
	// only hint at end of current word
	text := []rune(curLineText)
	if curCol < len(text) { return "" } // cursor not at line end

	// extract last identifier token
	i := len(text) - 1
	for i >= 0 && isIdentRune(text[i]) { i-- }
	word := string(text[i+1:])
	if word == "" { return "" }

	// exact match → show snippet
	if s, ok := snippets[word]; ok { return "→ " + s }

	// prefix match → suggest completion
	for _, kw := range kwList {
		if kw != word && strings.HasPrefix(kw, word) {
			return kw[len(word):] // show only the missing suffix
		}
	}
	return ""
}

func isIdentRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}

// ══════════════════════════════════════════════════════════════════════════════
//  evalREPL  — parse + run one input, auto-print expression results
// ══════════════════════════════════════════════════════════════════════════════

func evalREPL(interp *Interpreter, src string) {
	lexer := NewLexer(src, "<repl>")
	tokens, lexErrs := lexer.Tokenize()
	for _, e := range lexErrs { fmt.Print(e.Render()) }
	if len(lexErrs) > 0 { return }

	parser := NewParser(tokens, "<repl>", src)
	prog, parseErrs := parser.Parse()
	for _, e := range parseErrs { fmt.Print(e.Render()) }
	if len(parseErrs) > 0 { return }

	interp.file = "<repl>"
	interp.lines = strings.Split(src, "\n")

	// if the last statement is an expression, auto-print it
	if len(prog.Stmts) > 0 {
		if expr, ok := prog.Stmts[len(prog.Stmts)-1].(*ExprStmt); ok {
			for _, s := range prog.Stmts[:len(prog.Stmts)-1] {
				res := interp.execStmt(s, interp.globals)
				if res.Err != nil { printReplErr(res.Err); return }
				if res.Signal == SIG_RETURN { return }
			}
			res := interp.evalExpr(expr.Expr, interp.globals)
			if res.Err != nil { printReplErr(res.Err); return }
			if res.Value != nil && res.Value.Type != VAL_NULL {
				fmt.Printf("%s= %s%s%s\n", colorDim, colorGreen, res.Value.Repr(), colorReset)
			}
			return
		}
	}

	res := interp.execBlock(&BlockStmt{Stmts: prog.Stmts}, interp.globals)
	if res.Err != nil { printReplErr(res.Err) }
}

func printReplErr(err error) {
	if ve, ok := err.(*VexError); ok {
		fmt.Print(ve.Render())
	} else {
		fmt.Fprintf(os.Stderr, "%serror:%s %v\n", colorRed+colorBold, colorReset, err)
	}
}

// isIncomplete returns true if src has unclosed brackets/braces/parens
func isIncomplete(src string) bool {
	depth := 0
	inStr, strCh := false, rune(0)
	for _, ch := range src {
		if inStr {
			if ch == strCh { inStr = false }
		} else if ch == '"' || ch == '\'' || ch == '`' {
			inStr, strCh = true, ch
		} else if ch == '{' || ch == '(' || ch == '[' {
			depth++
		} else if ch == '}' || ch == ')' || ch == ']' {
			depth--
		}
	}
	return depth > 0
}

// ══════════════════════════════════════════════════════════════════════════════
//  MAIN REPL LOOP
// ══════════════════════════════════════════════════════════════════════════════

func runREPL() {
	fmt.Printf("%s%s%s", colorCyan, banner, colorReset)
	fmt.Printf("  %svex %s%s\n", colorBold, VERSION, colorReset)
	fmt.Printf("  %s↑↓ history · ←→ move · Ctrl+← → word · Ctrl+Enter / Alt+Enter newline · Ctrl+D exit%s\n\n",
		colorDim, colorReset)

	fd := int(os.Stdin.Fd())
	if err := termRaw(fd); err != nil {
		// not a tty (e.g. piped input) — fall back to line reader
		runPipedREPL()
		return
	}
	defer termRestore(fd)

	var (
		interp = NewInterpreter(nil)
		hist   = &history{}
		buf    = &lineBuf{}
		ren    = &renderer{}
	)

	// draw initial empty prompt
	ren.paint(buf, "")

	for {
		kv := readKey()

		switch kv.k {

		// ─── submit ───────────────────────────────────────────────────────────
		case kEnter:
			src := buf.String()
			trimmed := strings.TrimSpace(src)

			// Auto-continue: if braces are open, insert a newline instead
			if trimmed != "" && isIncomplete(src) {
				buf.insertRune('\n')
				ren.paint(buf, "")
				continue
			}

			ren.commit()
			termRestore(fd) // restore cooked mode so program output works

			if trimmed == "" {
				termRaw(fd)
				buf.reset()
				ren.paint(buf, "")
				continue
			}

			switch trimmed {
			case "exit", "quit":
				fmt.Printf("\n%sBye! 👋%s\n", colorCyan, colorReset)
				return
			case "help":
				printReplHelp()
			case "clear":
				fmt.Print("\033[H\033[2J")
			default:
				hist.push(src)
				evalREPL(interp, src)
			}

			hist.resetPos()
			termRaw(fd)
			buf.reset()
			ren.paint(buf, "")

		// ─── insert literal newline (multi-line editing) ──────────────────────
		case kNewline:
			buf.insertRune('\n')
			ren.paint(buf, "")

		// ─── cancel / interrupt ───────────────────────────────────────────────
		case kCtrlC:
			if buf.Len() == 0 {
				ren.commit()
				termRestore(fd)
				fmt.Printf("%s^C%s\n", colorDim, colorReset)
				termRaw(fd)
				ren.paint(buf, "")
			} else {
				buf.reset()
				ren.paint(buf, "")
			}

		// ─── exit ─────────────────────────────────────────────────────────────
		case kCtrlD:
			ren.commit()
			termRestore(fd)
			fmt.Printf("\n%sBye! 👋%s\n", colorCyan, colorReset)
			return

		// ─── clear screen ─────────────────────────────────────────────────────
		case kCtrlL:
			fmt.Print("\033[H\033[2J\033[3J") // clear + scrollback
			ren.prevRows = 0
			ren.paint(buf, "")

		// ─── cursor movement ──────────────────────────────────────────────────
		case kLeft:
			buf.moveLeft()
			ren.paint(buf, curHint(buf))
		case kRight:
			buf.moveRight()
			ren.paint(buf, curHint(buf))
		case kWordL:
			buf.moveWordLeft()
			ren.paint(buf, curHint(buf))
		case kWordR:
			buf.moveWordRight()
			ren.paint(buf, curHint(buf))
		case kHome:
			buf.moveToLineStart()
			ren.paint(buf, curHint(buf))
		case kEnd:
			buf.moveToLineEnd()
			ren.paint(buf, curHint(buf))

		// ─── history navigation (↑↓ only when single-line) ───────────────────
		case kUp:
			lines, cl, _ := buf.splitLines()
			if len(lines) > 1 && cl > 0 {
				// multi-line: move cursor up within buffer
				buf.moveLineUp()
				ren.paint(buf, "")
			} else {
				if e, ok := hist.prev(buf.String()); ok {
					buf.set(e)
					ren.paint(buf, "")
				}
			}
		case kDown:
			lines, cl, _ := buf.splitLines()
			if len(lines) > 1 && cl < len(lines)-1 {
				buf.moveLineDown()
				ren.paint(buf, "")
			} else {
				if e, ok := hist.next(); ok {
					buf.set(e)
					ren.paint(buf, "")
				}
			}

		// ─── editing ──────────────────────────────────────────────────────────
		case kBack:
			buf.backspace()
			ren.paint(buf, curHint(buf))
		case kDel:
			buf.deleteForward()
			ren.paint(buf, curHint(buf))
		case kKillEOL:
			buf.killToEOL()
			ren.paint(buf, curHint(buf))
		case kKillLine:
			buf.reset()
			ren.paint(buf, "")
		case kKillWordB:
			buf.killWordBack()
			ren.paint(buf, curHint(buf))

		// ─── tab completion ───────────────────────────────────────────────────
		case kTab:
			lines, cl, cc := buf.splitLines()
			if cl < len(lines) {
				h := hint(lines[cl], cc)
				if h != "" && !strings.HasPrefix(h, "→") {
					// h is just the suffix to append
					for _, r := range h {
						buf.insertRune(r)
					}
					ren.paint(buf, "")
				} else {
					buf.insertRune('\t')
					ren.paint(buf, "")
				}
			}

		// ─── regular character ────────────────────────────────────────────────
		case kRune:
			buf.insertRune(kv.r)
			ren.paint(buf, curHint(buf))

		case kUnknown:
			// ignore unknown escape sequences
		}
	}
}

// curHint returns the hint for the current cursor position
func curHint(b *lineBuf) string {
	lines, cl, cc := b.splitLines()
	if cl >= len(lines) { return "" }
	return hint(lines[cl], cc)
}

// ══════════════════════════════════════════════════════════════════════════════
//  PIPED / NON-TTY FALLBACK
// ══════════════════════════════════════════════════════════════════════════════

func runPipedREPL() {
	interp := NewInterpreter(nil)
	acc := strings.Builder{}
	lineRaw := make([]byte, 4096)
	for {
		fmt.Printf("%s❯%s ", colorCyan+colorBold, colorReset)
		n, err := os.Stdin.Read(lineRaw)
		if err != nil || n == 0 { fmt.Println(); return }
		acc.WriteString(strings.TrimRight(string(lineRaw[:n]), "\r\n") + "\n")
		src := acc.String()
		if !isIncomplete(src) {
			t := strings.TrimSpace(src)
			if t == "exit" || t == "quit" { return }
			evalREPL(interp, src)
			acc.Reset()
		}
	}
}

// ══════════════════════════════════════════════════════════════════════════════
//  HELP
// ══════════════════════════════════════════════════════════════════════════════

func printReplHelp() {
	fmt.Printf(`
%sKEYBINDINGS%s
  ←  →          move cursor left / right
  Ctrl+← →      jump word left / right
  ↑  ↓          history prev / next  (or move line in multi-line buffer)
  Home / End    line start / end  (also Ctrl+A / Ctrl+E)
  Backspace     delete char left
  Delete        delete char right
  Ctrl+K        kill to end of line
  Ctrl+U        kill entire buffer
  Ctrl+W        kill word left
  Alt+Enter     insert literal newline (start multi-line block)
  Ctrl+J        insert literal newline (same as Alt+Enter)
  Enter         submit  (auto-continues if braces unclosed)
  Tab           complete keyword / insert tab
  Ctrl+L        clear screen
  Ctrl+C        cancel current input
  Ctrl+D        exit

%sQUICK EXAMPLES%s
  let x = 42
  [1,2,3].map(fn(n) { n * 2 })
  struct Point { x=0, y=0 }
  try { risky() } catch e { print(e) }
  fetch "https://httpbin.org/get"
  $%s ls -la /tmp %s
  serve "0.0.0.0:8080" { GET "/" => fn(req,res){ res.send("hi") } }

%sCOMMANDS%s
  help    this message
  clear   clear screen
  exit    quit

`,
		colorBold+colorCyan, colorReset,
		colorBold+colorYellow, colorReset,
		"`", "`",
		colorBold+colorYellow, colorReset)
}
