# Zerverless Examples

## Wasm Modules

You need [TinyGo](https://tinygo.org/getting-started/install/) to compile Go examples.

### Add Example (Go/Wasm)

A simple module that adds two numbers.

```bash
cd examples/add
tinygo build -o add.wasm -target=wasi main.go
```

## Python Scripts

Python scripts run on MicroPython WASM. No compilation needed.

### Setup MicroPython

```bash
# Build MicroPython WASM from source
git clone https://github.com/nickovs/micropython-wasm.git
cd micropython-wasm
make
cp micropython.wasm /path/to/zerverless/bin/
```

### Python Examples

- `python/hello.py` - Simple addition
- `python/fibonacci.py` - Fibonacci sequence

### Python Input/Output

```python
# INPUT is a dict with job input_data
a = INPUT.get("a", 0)
b = INPUT.get("b", 0)

# Output via print (stdout captured as result)
print(a + b)
```

### Input/Output Contract

Wasm modules communicate with the host via these imported functions:

```go
// Get the length of input JSON
//go:wasmimport env get_input_len
func getInputLen() uint32

// Copy input JSON to Wasm memory
//go:wasmimport env get_input
func getInput(ptr unsafe.Pointer, size uint32)

// Send output JSON to host
//go:wasmimport env set_output
func setOutput(ptr unsafe.Pointer, size uint32)
```

### Required Export

Your module must export a `run` function (or `_start` for WASI):

```go
//export run
func run() {
    // Your logic here
}
```

## Example Usage

```bash
# Start orchestrator
./bin/zerverless

# Start worker
./bin/zerverless --worker ws://localhost:8000/ws/volunteer

# Submit job (once job submission is implemented)
curl -X POST http://localhost:8000/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "wasm_url": "http://localhost:8080/add.wasm",
    "input_data": {"a": 5, "b": 3}
  }'
```

Expected output: `{"result": 8}`

