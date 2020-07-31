package main

func main() {
	go runHTTPServer()
	runGRPCMockServer()
}