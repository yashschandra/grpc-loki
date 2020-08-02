package main

import (
	"encoding/binary"
	"encoding/json"
	"bytes"
	"reflect"
)

const (
	__string = "string"
	__int = "int"
	__float = "float"
	__double = "double"
	__bool = "bool"
	__dict = "dict"
	__array = "array"
)

type fieldData struct {
	Pos int `json:"pos"`
	Val interface{} `json:"val"`
	Typ string `json:"typ"`
	Repeated bool
}

var tagMap = map[string]int{
	__string: 2,
	__dict: 2,
	__array: 2,
	__int: 0,
	__float:5,
	__double:1,
}

type pbData map[string]fieldData

type _string string
type _int int64
type _float float32
type _double float64
type _bool bool
type _dict pbData

func isZeroValue (v interface{}) bool {
	return reflect.DeepEqual(v, reflect.Zero(reflect.TypeOf(v)).Interface())
}


func (s _string) encode() []byte {
	return []byte(s)
}

func (i _int) encode() []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	if int64(i) < 0 {
		i += _int(1<<62)
		i += _int(1<<62)
		i += _int(1<<62)
		i += _int(1<<62)
	}
	n := binary.PutUvarint(buf, uint64(i))
	return buf[:n]
}

func (f _float) encode() []byte {
	e := new(bytes.Buffer)
	binary.Write(e, binary.LittleEndian, float32(f))
	return e.Bytes()
}

func (d _double) encode() []byte {
	e := new(bytes.Buffer)
	binary.Write(e, binary.LittleEndian, float64(d))
	return e.Bytes()
}

func (b _bool) encode() []byte {
	if !b {
		return []byte{}
	}
	return []byte{1}
}

func (f fieldData) encodeRepeated() []byte {
	var b []byte
	switch f.Typ {
	case __string, __dict:
		for _, v := range f.Val.([]interface{}) {
				_f := fieldData{Pos: f.Pos, Val: v, Typ: f.Typ}
			b = append(b, _f.encodeWithTag()...)
		}
	case __float:
		for _, v := range f.Val.([]interface{}) {
			b = append(b, _float(v.(float64)).encode()...)
		}
		b = append(_int(len(b)).encode(), b...)
		b = append(fieldData{Typ: __array, Pos: f.Pos}.getTag(), b...)
	case __double:
		for _, v := range f.Val.([]interface{}) {
			b = append(b, _double(v.(float64)).encode()...)
		}
		b = append(_int(len(b)).encode(), b...)
		b = append(fieldData{Typ: __array, Pos: f.Pos}.getTag(), b...)
	case __int:
		for _, v := range f.Val.([]interface{}) {
			b = append(b, _int(v.(float64)).encode()...)
		}
		b = append(_int(len(b)).encode(), b...)
		b = append(fieldData{Typ: __array, Pos: f.Pos}.getTag(), b...)
	}
	return b
}

func (f fieldData) getTag() []byte {
	t := f.Pos << 3 | tagMap[f.Typ]
	return _int(t).encode()
}

func (f fieldData) encodeWithTag() []byte {
	if isZeroValue(f.Val) {
		return []byte{}
	}
	tag := f.getTag()
	var b []byte
	switch f.Typ {
	case __string:
		b = _string(f.Val.(string)).encode()
		b = append(_int(len(b)).encode(), b...)
	case __int:
		b = _int(f.Val.(float64)).encode()
	case __float:
		b = _float(f.Val.(float64)).encode()
	case __double:
		b = _double(f.Val.(float64)).encode()
	case __bool:
		b = _bool(f.Val.(bool)).encode()
	case __dict:
		var v pbData
		_b, _ := json.Marshal(f.Val)
		_ = json.Unmarshal(_b, &v)
		b = v.encode()
		b = append(_int(len(b)).encode(), b...)
	}
	return append(tag, b...)
}

func (d pbData) encode() []byte {
	var b []byte
	for _, v := range d {
		if _, ok := v.Val.([]interface{}); ok {
			b = append(b, v.encodeRepeated()...)
		} else {
			b = append(b, v.encodeWithTag()...)
		}
	}
	return b
}
