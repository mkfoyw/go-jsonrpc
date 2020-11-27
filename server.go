package jsonrpc

import "net/http"

type RPCServer struct {
}

func NewServer() *RPCServer {
	return &RPCServer{}
}

func (s *RPCServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

}
