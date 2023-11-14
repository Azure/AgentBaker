package main

import (
	"github.com/Azure/agentbaker/pkg/parser"
)

// input to this function will be the serialized JSON from userdata + custom data.
// it will be deserialized to the contract that the VHD this binary will be on supports.
// Parse will be called using that deserialized struct.
func main() {
	parser.Parse()
}
