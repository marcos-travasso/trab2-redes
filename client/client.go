package main

import (
	"log"
	"net"

	"golang.org/x/sys/unix"
)

func main() {
	// Create UDP socket
	clientFd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		log.Fatal("Error creating UDP socket:", err)
	}
	defer unix.Close(clientFd)

	log.Printf("UDP Socket [%v] created\n", clientFd)

	// Server address and port
	serverAddr := &unix.SockaddrInet4{
		Port: 8080,
	}
	copy(serverAddr.Addr[:], net.ParseIP("127.0.0.1").To4())

	// Message to send
	message := []byte("Hello, server!")

	// Send the message to the server
	err = unix.Sendto(clientFd, message, 0, serverAddr)
	if err != nil {
		log.Fatal("Error sending message to server:", err)
	}

	log.Printf("Message sent to the server: %s\n", string(message))

	// Receive the response from the server
	buffer := make([]byte, 1024)
	n, _, err := unix.Recvfrom(clientFd, buffer, 0)
	if err != nil {
		log.Fatal("Error receiving response from server:", err)
	}

	log.Printf("Received %d bytes from server: %s\n", n, string(buffer[:n]))
}
