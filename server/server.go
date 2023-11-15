package main

import (
	"log"
	"net"

	"golang.org/x/sys/unix"
)

func main() {
	// Create UDP socket
	serverFd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		log.Fatal("Error creating UDP socket:", err)
	}
	defer unix.Close(serverFd)

	log.Printf("UDP Socket [%v] created\n", serverFd)

	// Bind the socket to a specific address and port
	socketAddress := &unix.SockaddrInet4{
		Port: 8080,
	}
	copy(socketAddress.Addr[:], net.ParseIP("0.0.0.0").To4())

	err = unix.Bind(serverFd, socketAddress)
	if err != nil {
		log.Fatal("Error binding UDP socket:", err)
	}
	log.Printf("UDP Socket binded to port %v\n", socketAddress.Port)

	// Handle incoming data
	buffer := make([]byte, 1024)
	for {
		n, addr, err := unix.Recvfrom(serverFd, buffer, 0)
		if err != nil {
			log.Println("Error reading from UDP socket:", err)
			continue
		}

		clientAddr := addr.(*unix.SockaddrInet4)

		log.Printf("Received %d bytes from %+v: %s\n", n, clientAddr, string(buffer[:n]))

		// Respond to the client
		response := []byte("Hello, client!")
		err = unix.Sendto(serverFd, response, 0, &unix.SockaddrInet4{
			Port: clientAddr.Port,
		})
		if err != nil {
			log.Println("Error writing to UDP socket:", err)
		}
	}
}
