package jsonrpc

import (
	"fmt"
	"reflect"
)

type ErrClient struct {
	err error
}

func (e *ErrClient) Error() string {
	return fmt.Sprintf("RPC client error: %s", e.err)
}

func (e *ErrClient) Unwrap(err error) error {
	return e.err
}

type resut reflect.Value
