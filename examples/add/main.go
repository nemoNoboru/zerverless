//go:build tinygo

package main

import (
	"encoding/json"
	"unsafe"
)

// Imports from host
//
//go:wasmimport env get_input
func getInput(ptr unsafe.Pointer, size uint32)

//go:wasmimport env get_input_len
func getInputLen() uint32

//go:wasmimport env set_output
func setOutput(ptr unsafe.Pointer, size uint32)

type Input struct {
	A int `json:"a"`
	B int `json:"b"`
}

type Output struct {
	Result int `json:"result"`
}

//export run
func run() {
	// Get input
	size := getInputLen()
	buf := make([]byte, size)
	getInput(unsafe.Pointer(&buf[0]), size)

	var input Input
	json.Unmarshal(buf, &input)

	// Compute
	output := Output{Result: input.A + input.B}

	// Set output
	outBuf, _ := json.Marshal(output)
	setOutput(unsafe.Pointer(&outBuf[0]), uint32(len(outBuf)))
}

func main() {}


