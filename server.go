package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
	"io"
	"log"
	"net"
	"net/http"
)

func runGRPCMockServer() {
	l, err := net.Listen("tcp",":50000")
	fmt.Println("listening...")
	if err != nil {
		log.Fatal("could not listen over tcp:", err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalln(err)
		}
		go func(conn net.Conn) {
			defer conn.Close()

			const preface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
			b := make([]byte, len(preface))
			if _, err := io.ReadFull(conn, b); err != nil {
				log.Fatalln(err)
			}
			if string(b) != preface {
				log.Fatalln("invalid preface: ", string(b))
			}

			framer := http2.NewFramer(conn, conn)
			handleGRPCRequest(framer)
		}(conn)

	}
}

func getDataFromFrame(frame *http2.DataFrame) []byte {
	return frame.Data()
}

func getPathFromHeaders(frame *http2.HeadersFrame) string {
	decoder := hpack.NewDecoder(2048, nil)
	hf, _ := decoder.DecodeFull(frame.HeaderBlockFragment())
	for _, h := range hf {
		if h.Name == ":path" {
			return h.Value[1:]
		}
	}
	return ""
}

func getSettingsFromFrame(frame *http2.SettingsFrame) []http2.Setting {
	var settings []http2.Setting
	frame.ForeachSetting(func(setting http2.Setting) error {
		if setting.Val != 0 {
			settings = append(settings, setting)
		}
		return nil
	})
	return settings
}


func handleGRPCRequest(framer *http2.Framer) {
	var reqData []byte
	var path string
	var settings []http2.Setting
	for {
		frame, _ := framer.ReadFrame()
		if frame == nil {
			break
		}
		var streamEnded bool
		switch frame.Header().Type {
		case http2.FrameData:
			reqData = getDataFromFrame(frame.(*http2.DataFrame))
			streamEnded = frame.(*http2.DataFrame).StreamEnded()
		case http2.FrameHeaders:
			path = getPathFromHeaders(frame.(*http2.HeadersFrame))
			streamEnded = frame.(*http2.HeadersFrame).StreamEnded()
		case http2.FrameSettings:
			settings = getSettingsFromFrame(frame.(*http2.SettingsFrame))
		}
		if streamEnded {
			break
		}
	}
	respData := get(path, reqData[5:])
	returnGRPCResponse(framer, respData, settings)
}

func returnGRPCResponse(framer *http2.Framer, respData []byte, settings []http2.Setting) {
	framer.WriteSettings(settings...)
	framer.WriteWindowUpdate(0, 1234)
	framer.WritePing(false, [8]byte{0, 0, 0, 0, 0, 0, 0, 0})
	framer.WriteSettingsAck()
	hbuf := bytes.NewBuffer([]byte{})
	encoder := hpack.NewEncoder(hbuf)
	encoder.WriteField(hpack.HeaderField{Name: ":status", Value: "200"})
	encoder.WriteField(hpack.HeaderField{Name: "content-type", Value: "application/grpc"})
	encoder.WriteField(hpack.HeaderField{Name: "grpc-accept-encoding", Value: "identity,deflate,gzip"})
	encoder.WriteField(hpack.HeaderField{Name: "accept-encoding", Value: "identity,gzip"})
	framer.WriteHeaders(http2.HeadersFrameParam{StreamID: 1, BlockFragment: hbuf.Bytes(), EndHeaders: true})
	framer.WriteData(1, false, respData)
	hbuf = bytes.NewBuffer([]byte{})
	encoder = hpack.NewEncoder(hbuf)
	encoder.WriteField(hpack.HeaderField{Name: "grpc-status", Value: "0"})
	encoder.WriteField(hpack.HeaderField{Name: "grpc-message"})
	framer.WriteHeaders(http2.HeadersFrameParam{StreamID: 1, BlockFragment: hbuf.Bytes(), EndHeaders: true, EndStream: true})
}

func setExpectation(w http.ResponseWriter, r *http.Request) {
	type data struct {
		Path string `json:"path"`
		Request pbData `json:"request"`
		Response pbData `json:"response"`
	}
	var d data
	err := json.NewDecoder(r.Body).Decode(&d)
	if err != nil {
		panic(err)
	}
	reqBytes := d.Request.encode()
	respBytes := d.Response.encode()
	respLen := len(respBytes)
	bs := make([]byte, 4)
	binary.BigEndian.PutUint32(bs, uint32(respLen))
	respBytes = append(bs, respBytes...)
	respBytes = append([]byte{0}, respBytes...)
	err = add(d.Path, reqBytes, respBytes)
	if err != nil {
		panic(err)
	}
}

func runHTTPServer() {
	http.HandleFunc("/set", setExpectation)
	log.Fatal(http.ListenAndServe(":51000", nil))
}
