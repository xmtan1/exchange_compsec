package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
)

const defaultServerDirectory = "."
const defaultServerPort = 30022

func StartHTTPFileServer(rootDirectory string, serverPort int) error {
	_, err := os.Stat(rootDirectory)
	if os.IsNotExist(err) {
		fmt.Printf("Directory %s is either not accessible or not exist.\n", rootDirectory)
		return err
	}

	fileServer := http.FileServer(http.Dir(rootDirectory)) // start a fileserver at rootDirectory

	// handle http
	http.Handle("/", fileServer)

	err = http.ListenAndServe(fmt.Sprintf(":%d", serverPort), nil)
	if err != nil {
		fmt.Printf("Error starting the server %s", err)
		return err
	}
	return nil
}

func GetServerAddress() (myAdress string) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	return ""
}

func main() {
	fmt.Println("Starting the server..., try to use the constants first...")

	// sigChannel := make(chan os.Signal, 1) // channel to handle the shutdown signal
	// signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	// <-sigChannel

	//for {
	go func() {
		if err := StartHTTPFileServer(defaultServerDirectory, defaultServerPort); err != nil {
			fmt.Printf("Server failed: %v", err)
			os.Exit(1)
		}
	}()

	serverAddress := GetServerAddress()
	if serverAddress == "" {
		fmt.Println("Failed to get server's address, something went wrong!")
		os.Exit(1)
	}

	fmt.Printf("Successfully started a server at %s with port %d...", serverAddress, defaultServerPort)

	//}

	// shutdown server
	select {}
}
