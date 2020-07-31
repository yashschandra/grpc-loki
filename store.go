package main

import (
	"bytes"
)

type expectation struct {
	reqData []byte
	respData []byte
}

var store map[string]expectation

func init() {
	store = make(map[string]expectation)
}

func add(path string, reqData []byte, respData []byte) error {
	store[path] = expectation{
		reqData:  reqData,
		respData: respData,
	}
	return nil
}

func get(path string, reqData []byte) []byte {
	p := store[path]
	if bytes.Compare(p.reqData, reqData) == 0 {
		return p.respData
	}
	return nil
}
