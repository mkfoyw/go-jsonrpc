package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
)

const (
	rpcParseError     = -32700
	rpcMethodNotFound = -32601
	rpcInvalidParams  = 32602
)

var contextType = reflect.TypeOf(new(context.Context)).Elem()
var errorType = reflect.TypeOf(new(error)).Elem()

type rpcHandler struct {
	paramsReceiver []reflect.Type
	nParams        int

	reciver    reflect.Value
	handleFunc reflect.Value

	hasCtx int

	errOut int
	valOut int
}

type RPCServer struct {
	methods map[string]rpcHandler
}

func NewServer() *RPCServer {
	return &RPCServer{
		methods: map[string]rpcHandler{},
	}
}

type request struct {
	Jsonrpc string  `json:"jsonrpc"`
	ID      *int64  `json:"id,omitempty"`
	Method  string  `json:"method"`
	Params  []param `json:"params"`
}

type param struct {
	data []byte
	val  reflect.Value
}

func (p *param) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.val.Interface())
}

func (p *param) UnmarshalJSON(d []byte) error {
	raw := make([]byte, len(d))
	copy(raw, d)
	p.data = raw
	return nil
}

type respErr struct {
	Code    int
	Message string
}

//
type response struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int64  `json:"id"`

	Result interface{} `json:"result,omitempty"`
	Error  *respErr    `json:"error,omitempty"`
}

func (s *RPCServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.rpcError(w, req, rpcParseError, err)
	}
	handler, ok := s.methods[req.Method]
	if !ok {
		s.rpcError(w, req, rpcMethodNotFound, fmt.Errorf("Method not found"))
		return
	}

	if len(req.Params) != handler.nParams {
		s.rpcError(w, req, rpcInvalidParams, fmt.Errorf("wrong params count"))
		return
	}

	// 组装调用参数
	callParams := make([]reflect.Value, handler.nParams+handler.hasCtx+1)
	callParams[0] = handler.reciver
	if handler.hasCtx == 1 {
		callParams[1] = reflect.ValueOf(r.Context())
	}

	for i := 0; i < handler.nParams; i++ {
		rp := reflect.New(handler.paramsReceiver[i])
		if err := json.NewDecoder(bytes.NewReader(req.Params[i].data)).Decode(rp.Interface()); err != nil {
			s.rpcError(w, req, rpcParseError, err)
			return
		}
		callParams[i+1+handler.hasCtx] = rp
	}

	//调用 handler
	callResult := handler.handleFunc.Call(callParams)
	if req.ID == nil {
		return
	}

	resp := response{
		Jsonrpc: Version,
		ID:      *req.ID,
	}

	if handler.errOut != -1 {
		err := callResult[handler.errOut].Interface()
		if err != nil {
			resp.Error = &respErr{
				Code:    1,
				Message: err.(error).Error(),
			}
		}
	}

	if handler.valOut != -1 {
		resp.Result = callResult[handler.valOut]
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		fmt.Println(err)
		return
	}
}

func (s *RPCServer) rpcError(w http.ResponseWriter, r request, code int, err error) {
	w.WriteHeader(500)
	if r.ID == nil {
		return
	}

	resp := response{
		Jsonrpc: Version,
		ID:      *r.ID,
		Error: &respErr{
			Code:    code,
			Message: err.Error(),
		},
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		fmt.Println(err)
		return
	}
}

func (s *RPCServer) Regiter(namespace string, handler interface{}) {
	htyp := reflect.TypeOf(handler)
	if htyp.Kind() != reflect.Ptr {
		panic("handler should be a pointer")
	}

	if htyp.Elem().Kind() != reflect.Struct {
		panic("handler should be a struct")
	}

	val := reflect.ValueOf(handler)
	for i := 0; i < val.NumMethod(); i++ {
		method := val.Type().Method(i)
		funcType := method.Type

		hasCtx := 0
		if funcType.NumIn() >= 2 && funcType.In(1) == contextType {
			hasCtx = 1
		}

		ins := funcType.NumIn() - 1 - hasCtx

		recvs := make([]reflect.Type, ins)

		for i := 0; i < ins; i++ {
			recvs[i] = funcType.In(i)
		}
		valOut, errOut, _ := processOutput(funcType)
		fmt.Println(namespace + "." + method.Name)

		s.methods[namespace+"."+method.Name] = rpcHandler{
			paramsReceiver: recvs,
			nParams:        ins,

			reciver:    val,
			handleFunc: method.Func,

			hasCtx: hasCtx,

			errOut: errOut,
			valOut: valOut,
		}

	}
}

func processOutput(funcType reflect.Type) (valOut int, errOut int, n int) {
	valOut = -1
	errOut = -1
	n = funcType.NumOut()
	switch n {
	case 0:
	case 1:
		if funcType.Out(0) == errorType {
			errOut = 0
		} else {
			valOut = 0
		}
	case 2:
		valOut = 0
		errOut = 1
		if funcType.Out(1) != errorType {
			panic("expected error as second return value")
		}
	default:
		panic("too many error values")
	}
	return
}
