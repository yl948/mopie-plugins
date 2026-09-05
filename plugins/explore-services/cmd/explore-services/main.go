package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}
type rpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	r := newRuntime()
	serve(r, os.Stdin, os.Stdout)
}

func serve(r *runtime, in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	encoder := json.NewEncoder(out)
	for scanner.Scan() {
		var request rpcRequest
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			writeRPC(encoder, request.ID, nil, &rpcError{Code: -32700, Message: "invalid json"})
			continue
		}
		if request.Method == "shutdown" {
			writeRPC(encoder, request.ID, map[string]string{"status": "ok"}, nil)
			return
		}
		result, err := r.handle(request.Method, request.Params)
		if err != nil {
			writeRPC(encoder, request.ID, nil, &rpcError{Code: -32000, Message: err.Error()})
			continue
		}
		writeRPC(encoder, request.ID, result, nil)
	}
}

func writeRPC(encoder *json.Encoder, id interface{}, result interface{}, rpcErr *rpcError) {
	_ = encoder.Encode(rpcResponse{JSONRPC: "2.0", ID: id, Result: result, Error: rpcErr})
}

func (r *runtime) handle(method string, raw json.RawMessage) (interface{}, error) {
	switch method {
	case "initialize":
		return r.initialize(raw)
	case "health.check":
		return r.healthResult(), nil
	case "tool.call":
		return r.callTool(raw)
	default:
		return nil, fmt.Errorf("method not found: %s", method)
	}
}

func (r *runtime) callTool(raw json.RawMessage) (interface{}, error) {
	var call struct {
		ToolName string                 `json:"toolName"`
		Args     map[string]interface{} `json:"args"`
	}
	if err := json.Unmarshal(raw, &call); err != nil {
		return nil, err
	}
	result, err := r.tool(call.ToolName, call.Args)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"result": result}, nil
}
