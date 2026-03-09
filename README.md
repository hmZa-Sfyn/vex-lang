# vex-lang 🌐

**A fast, network-first scripting language built in Go.**

vex is purpose-built for networking — HTTP servers, TCP/UDP sockets, WebSockets, concurrent requests, and more. It has Rust-style error messages with underlines and fix hints, a pipe operator, closures, async/spawn, and a clean modern syntax.

```
 ██╗   ██╗███████╗██╗  ██╗      ██╗      █████╗ ███╗   ██╗ ██████╗
 ██║   ██║██╔════╝╚██╗██╔╝      ██║     ██╔══██╗████╗  ██║██╔════╝
 ██║   ██║█████╗   ╚███╔╝ █████╗██║     ███████║██╔██╗ ██║██║  ███╗
 ╚██╗ ██╔╝██╔══╝   ██╔██╗ ╚════╝██║     ██╔══██║██║╚██╗██║██║   ██║
  ╚████╔╝ ███████╗██╔╝ ██╗      ███████╗██║  ██║██║ ╚████║╚██████╔╝
   ╚═══╝  ╚══════╝╚═╝  ╚═╝      ╚══════╝╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝
```

---

## Build

```bash
go build -o vex .
```

> All source files are `package main` — single binary, no external dependencies.

---

## Usage

```bash
vex run script.vex        # run a script
vex repl                  # interactive REPL
vex check script.vex      # syntax check only
vex --version             # print version
```

---

## Language Tour

### Variables
```vex
let name = "Alice"
const MAX = 100
let x = 42
```

### Strings & Templates
```vex
let msg = "Hello, ${name}!"
let upper = name.upper()
let has = msg.contains("Alice")
msg.split(" ")
msg.replace("Hello", "Hi")
msg.trim()
msg.slice(0, 5)
```

### Arrays
```vex
let nums = [1, 2, 3, 4, 5]
nums.push(6)
nums.map(fn(n) { n * 2 })
nums.filter(fn(n) { n > 2 })
nums.reduce(fn(acc, n) { acc + n }, 0)
nums.find(fn(n) { n > 3 })
nums.includes(3)
nums.join(", ")
nums.slice(0, 3)
nums.reverse()
nums.first
nums.last
nums.len
```

### Objects
```vex
let user = {
  name: "Alice",
  age: 30,
  roles: ["admin", "user"]
}

user.name           // "Alice"
user["age"]         // 30
user.roles.len      // 2

keys(user)          // ["name", "age", "roles"]
values(user)
entries(user)
```

### Functions & Closures
```vex
fn greet(name, prefix = "Hello") {
  return "${prefix}, ${name}!"
}

let double = fn(x) { x * 2 }

fn make_adder(n) {
  return fn(x) { x + n }
}

let add5 = make_adder(5)
add5(10)  // 15
```

### Pipe Operator `|>`
```vex
let result = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
  |> fn(a) { a.filter(fn(n) { n % 2 == 0 }) }
  |> fn(a) { a.map(fn(n) { n * n }) }
  |> fn(a) { a.reduce(fn(acc, n) { acc + n }, 0) }

// Or using named functions:
result |> double |> str |> print
```

### Control Flow
```vex
if x > 10 {
  print("big")
} else if x > 5 {
  print("medium")
} else {
  print("small")
}

while i < 10 {
  i += 1
}

for item in array { ... }
for key, val in object { ... }
for i in range(0, 10) { ... }

break
continue
```

### Match (Pattern Matching)
```vex
match status_code {
  200 => { return "OK" }
  404 => { return "Not Found" }
  500 => { return "Server Error" }
  _   => { return "Unknown" }
}
```

### Async / Spawn
```vex
fn slow_task(n) {
  time.sleep(100)
  return n * 2
}

// Spawn runs concurrently (goroutine)
let p1 = spawn slow_task(1)
let p2 = spawn slow_task(2)

// Await blocks until done
let r1 = await p1
let r2 = await p2
```

### Null Coalescing & Ternary
```vex
let val = might_be_null ?? "default"
let label = x > 0 ? "positive" : "non-positive"
```

### Error Handling
```vex
let res = fetch "https://example.com"
if is_err(res) {
  print("error:", err_msg(res))
}

// Null coalescing with errors
let data = risky_call() ?? fallback_value
```

---

## Network Primitives

### HTTP Fetch
```vex
// GET
let res = fetch "https://api.example.com/users"
print(res.status)         // 200
print(res.ok)             // true
let body = res.text()     // read as string
let data = res.json()     // parse as JSON

// POST with body
let resp = fetch "https://api.example.com/users" {
  method: "POST",
  body: { name: "Alice", role: "admin" },
  headers: { "Authorization": "Bearer token123" }
}

// PATCH / PUT / DELETE
let r = fetch url { method: "DELETE" }
```

### HTTP Server
```vex
serve "0.0.0.0:3000" {
  GET "/" => fn(req, res) {
    res.json({ hello: "world" })
  }

  GET "/users" => fn(req, res) {
    let search = req.query.name   // ?name=...
    res.json(users)
  }

  POST "/users" => fn(req, res) {
    let body = req.json()
    res.status(201)
    res.json(body)
  }

  * "/api/anything" => fn(req, res) {   // all methods
    res.html("<h1>Hi</h1>")
  }
}
```

#### Request object
```vex
req.method       // "GET", "POST", ...
req.url          // full URL string
req.path         // /path/only
req.query        // { key: "value" } from ?key=value
req.headers      // { "content-type": "..." }
req.body()       // read raw body string
req.json()       // parse body as JSON
req.remote_addr  // client IP
```

#### Response object
```vex
res.send("plain text")
res.json({ key: "value" })
res.html("<h1>hi</h1>")
res.status(404)
res.header("X-Custom", "value")
```

### TCP
```vex
// Server
let server = listen "0.0.0.0:9000"

while true {
  let conn = net.accept(server)
  spawn fn() {
    let data = recv conn
    send conn "echo: " + data
    net.close(conn)
  }()
}

// Client
let conn = connect "localhost:9000"
send conn "hello!\n"
let reply = recv conn
net.close(conn)
```

### TCP Connection Object Methods
```vex
net.accept(listener)       // accept next connection
net.close(conn)            // close connection or listener
net.send(conn, data)       // send data
net.recv(conn)             // receive data (up to 4096 bytes)
net.recv(conn, 8192)       // custom buffer size
net.remote_addr(conn)      // "1.2.3.4:5678"
net.local_addr(conn)       // "0.0.0.0:9000"
```

### Channels (for manual async)
```vex
let ch = chan()         // unbuffered
let ch = chan(10)       // buffered

chan_send(ch, value)
let val = chan_recv(ch)

// Or via spawn/await pattern
```

---

## Built-in Modules

### `math`
```vex
math.pi, math.e, math.inf, math.nan
math.floor(x), math.ceil(x), math.round(x)
math.abs(x), math.sqrt(x), math.pow(x, y)
math.min(a, b), math.max(a, b)
math.log(x), math.sin(x), math.cos(x)
math.random()          // 0.0..1.0
math.rand_int(min, max)
```

### `json`
```vex
json.parse(str)              // string → value
json.stringify(val)          // value → compact JSON string
json.stringify(val, true)    // pretty-printed JSON
```

### `time`
```vex
time.now()                   // unix ms
time.sleep(ms)               // sleep milliseconds
time.format(ms, layout)      // format timestamp
```

### `os`
```vex
os.args                      // [string] command-line args
os.env("HOME")               // read env var
os.exit(code)                // exit process
os.read_file("path.txt")     // → string or error
os.write_file("path", data)  // → true or error
```

### `string`
```vex
string.from_bytes([72, 101, 108, 108, 111])   // "Hello"
string.repeat("ab", 3)                        // "ababab"
```

---

## Error Messages

vex produces Rust-style errors with source location, underlines, hints, and fix suggestions:

```
error[SyntaxError]: unexpected character '&'
  --> script.vex:12:8
   |
11 |   let x = 5
12 |   if x & 1 {
   |        ^ single '&' is not valid in vex
   |

  💡 help: use '&&' for logical and, or '|>' for pipe
```

```
error[UndefinedError]: 'usres' is not defined
  --> server.vex:24:12
   |
23 |   // serve users
24 |   res.json(usres)
   |            ^^^^^ variable not found in scope
   |

  💡 help: declare it with: let usres = ...
```

---

## File Structure

```
vex-lang/
├── main.go         ← entry point, REPL, CLI
├── token.go        ← token types and keyword map
├── lexer.go        ← tokenizer/lexer
├── ast.go          ← AST node definitions
├── parser.go       ← Pratt parser
├── value.go        ← runtime value types + environment
├── methods.go      ← built-in string/array/response methods
├── interpreter.go  ← tree-walk interpreter + built-ins
├── errors.go       ← pretty error rendering
├── json.go         ← JSON parse/stringify + template interpolation
├── go.mod
└── examples/
    ├── http_server.vex
    ├── tcp_echo.vex
    ├── fetch_demo.vex
    └── language_tour.vex
```

All files are `package main` for simplicity — single `go build` produces one binary.

---

## License

MIT
