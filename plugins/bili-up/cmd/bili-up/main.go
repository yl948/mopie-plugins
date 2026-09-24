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

func serve(rt *runtime, in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	enc := json.NewEncoder(out)
	for scanner.Scan() {
		var req rpcRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			writeRPC(enc, req.ID, nil, &rpcError{Code: -32700, Message: "无效的 JSON 请求"})
			continue
		}
		if req.Method == "shutdown" {
			rt.stop()
			writeRPC(enc, req.ID, map[string]string{"status": "ok"}, nil)
			return
		}
		result, err := rt.handle(req.Method, req.Params)
		if err != nil {
			writeRPC(enc, req.ID, nil, &rpcError{Code: -32000, Message: err.Error()})
			continue
		}
		writeRPC(enc, req.ID, result, nil)
	}
}

func writeRPC(enc *json.Encoder, id interface{}, result interface{}, rpcErr *rpcError) {
	_ = enc.Encode(rpcResponse{JSONRPC: "2.0", ID: id, Result: result, Error: rpcErr})
}

func logf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[bili-up] "+format+"\n", args...)
}
