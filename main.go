package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
	"io"
	"log"
	"net"
)

func run() {
	l, err := net.Listen("tcp",":50000")
	fmt.Println("listening...")
	if err != nil {
		log.Fatal("could not listen for tls over tcp:", err)
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
			handle(framer)
			//conn.Write([]byte("\x00\x00\x18\x04\x00\x00\x00\x00\x00\x00\x04\x00@\x00\x00\x00\x05\x00@\x00\x00\x00\x06\x00\x00 \x00\xfe\x03\x00\x00\x00\x01\x00\x00\x04\x08\x00\x00\x00\x00\x00\x00?\x00\x01\x00\x00\x08\x06\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x04\x01\x00\x00\x00\x00\x00\x00k\x01\x04\x00\x00\x00\x01\x88@\x0ccontent-type\x10application/grpc@\x14grpc-accept-encoding\x15identity,deflate,gzip@\x0faccept-encoding\ridentity,gzip\x00\x00\x15\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x10\n\x0eHello, hohoho!\x00\x00\x1e\x01\x05\x00\x00\x00\x01@\x0bgrpc-status\x010@\x0cgrpc-message\x00\x00\x00\x04\x08\x00\x00\x00\x00\x00\x00\x00\x00\r"))
		}(conn)

	}


}

func readDataFrame(dataFrame *http2.DataFrame) bool {
	fmt.Println("data ->", dataFrame.Data())
	return dataFrame.StreamEnded()
}

func writeHeaders1(framer *http2.Framer) {
	hbuf := bytes.NewBuffer([]byte{})
	encoder := hpack.NewEncoder(hbuf)
	encoder.WriteField(hpack.HeaderField{Name: ":status", Value: "200"})
	encoder.WriteField(hpack.HeaderField{Name: "content-type", Value: "application/grpc"})
	encoder.WriteField(hpack.HeaderField{Name: "grpc-accept-encoding", Value: "identity,deflate,gzip"})
	encoder.WriteField(hpack.HeaderField{Name: "accept-encoding", Value: "identity,gzip"})
	framer.WriteHeaders(http2.HeadersFrameParam{StreamID: 1, BlockFragment: hbuf.Bytes(), EndHeaders: true})
}

func writeHeaders2(framer *http2.Framer) {
	hbuf := bytes.NewBuffer([]byte{})
	encoder := hpack.NewEncoder(hbuf)
	encoder.WriteField(hpack.HeaderField{Name: "grpc-status", Value: "0"})
	encoder.WriteField(hpack.HeaderField{Name: "grpc-message"})
	framer.WriteHeaders(http2.HeadersFrameParam{StreamID: 1, BlockFragment: hbuf.Bytes(), EndHeaders: true, EndStream: true})
}

func readHeaderFrame(headerFrame *http2.HeadersFrame) bool {
	decoder := hpack.NewDecoder(2048, nil)
	hf, _ := decoder.DecodeFull(headerFrame.HeaderBlockFragment())
	for _, h := range hf {
		fmt.Println(h.Name + ":" + h.Value, h.Sensitive)
	}
	return headerFrame.StreamEnded()
}

func readWindowUpdate(windowframe *http2.WindowUpdateFrame) bool {
	fmt.Println("window update ->", windowframe.String(), windowframe.Increment)
	return false
}

func readSettingsFrame(settingsFrame *http2.SettingsFrame) (bool, []http2.Setting) {
	var settings []http2.Setting
	fmt.Println("settings ->", settingsFrame.String(), settingsFrame.IsAck(), settingsFrame.ForeachSetting(func(setting http2.Setting) error {
		fmt.Println("setting value ->", setting.String())
		if setting.Val != 0 {
			settings = append(settings, setting)
		}
		return nil
	}))
	return false, settings
}

func readPingFrame(pingframe *http2.PingFrame) bool {
	fmt.Println("ping ->", pingframe.String(), pingframe.Data, pingframe.IsAck())
	return false
}

func handle(framer *http2.Framer) {
	var settings []http2.Setting
	for {
		frame, err := framer.ReadFrame()
		if frame == nil {
			break
		}
		if err != nil {
			fmt.Println("error ->", err)
		}
		fmt.Println("frame type ->", frame.Header().Type, frame.Header().StreamID)
		var streamEnded bool
		switch frame.Header().Type {
		case http2.FrameData:
			streamEnded = readDataFrame(frame.(*http2.DataFrame))
		case http2.FrameHeaders:
			streamEnded = readHeaderFrame(frame.(*http2.HeadersFrame))
		case http2.FrameWindowUpdate:
			streamEnded = readWindowUpdate(frame.(*http2.WindowUpdateFrame))
		case http2.FrameSettings:
			streamEnded, settings = readSettingsFrame(frame.(*http2.SettingsFrame))
			if !frame.(*http2.SettingsFrame).IsAck() {
				framer.WriteSettings(settings...)
			}
		case http2.FramePing:
			streamEnded = readPingFrame(frame.(*http2.PingFrame))
		}
		if streamEnded {
			break
		}
	}

	framer.WriteWindowUpdate(0, 1234)
	framer.WritePing(false, [8]byte{0, 0, 0, 0, 0, 0, 0, 0})
	framer.WriteSettingsAck()
	writeHeaders1(framer)
	framer.WriteData(1, false, []byte{0, 0, 0, 0, 16, 10, 14, 72, 101, 108, 108, 111, 44, 32, 104, 111, 104, 111, 104, 111, 33})
	writeHeaders2(framer)
}

func main() {
	go runHTTPServer()
	runGRPCMockServer()
	//run()

	//l, err := net.Listen("tcp",":8080")
	//if err != nil {
	//	log.Fatal("could not listen for tls over tcp:", err)
	//}
	//defer l.Close()
	//fmt.Println("server started...")
	//
	//readFrames := func(framer *http2.Framer) ([]http2.Frame, error) {
	//	frames := make([]http2.Frame, 0)
	//	for {
	//		frame, err := framer.ReadFrame()
	//		if err != nil {
	//			return frames, err
	//		}
	//		frames = append(frames, frame)
	//		if frame.Header().Flags.Has(http2.FlagDataEndStream) {
	//			return frames, nil
	//		}
	//	}
	//}
	//
	//for {
	//	conn, err := l.Accept()
	//	if err != nil {
	//		log.Fatal("could not accept connection:", err)
	//	}
	//	defer conn.Close()
	//
	//	// Every connection starts with a connection preface send first, which has to be read prior
	//	// to reading any frames (RFC 7540, section 3.5)
	//	const preface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
	//	fmt.Println(preface)
	//	b := make([]byte, len(preface))
	//	if _, err := io.ReadFull(conn, b); err != nil {
	//		log.Fatal("could not read from connection:", err)
	//	}
	//	if string(b) != preface {
	//		log.Fatal("invalid preface")
	//	}
	//
	//	framer := http2.NewFramer(conn, conn)
	//	fmt.Println("framer ->", framer);
	//
	//	// Read client request (SETTINGS and HEADERS)
	//	readFrames(framer)
	//
	//	// Send empty SETTINGS frame to the client
	//	framer.WriteRawFrame(http2.FrameSettings, 0, 0, []byte{})
	//
	//	// Read clients response (contains empty SETTINGS with END_STREAM flag)
	//	readFrames(framer)
	//
	//	// Prepare HEADERS
	//	hbuf := bytes.NewBuffer([]byte{})
	//	encoder := hpack.NewEncoder(hbuf)
	//	encoder.WriteField(hpack.HeaderField{Name: ":status:", Value: "200"})
	//	encoder.WriteField(hpack.HeaderField{Name: "date", Value: time.Now().UTC().Format(http.TimeFormat)})
	//	encoder.WriteField(hpack.HeaderField{Name: "content-length", Value: strconv.Itoa(len("ok"))})
	//	encoder.WriteField(hpack.HeaderField{Name: "content-type", Value: "text/html"})
	//
	//	// Write HEADERS frame
	//	err = framer.WriteHeaders(http2.HeadersFrameParam{StreamID: 2, BlockFragment: hbuf.Bytes(), EndHeaders: true})
	//	if err != nil {
	//		log.Fatal("could not write headers: ", err)
	//	}
	//
	//	// Clients response contains GOAWAY
	//	readFrames(framer)
	//
	//	framer.WriteData(2, true, []byte("ok"))
	//	conn.Close()

	var d pbData
	_ = json.Unmarshal([]byte(data), &d)
	fmt.Println(d.encode())

}