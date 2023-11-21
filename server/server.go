package main

import (
	"encoding/binary"
	"errors"
	"golang.org/x/sys/unix"
	"log"
	"net"
	"strings"
)

var PORT = 8080
var serverFd int
var BUFFER_SIZE = 2048

var COMMAND_ERROR = errors.New("command error")
var FILE_ERROR = errors.New("file error")

func main() {
	var err error
	serverFd, err = unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		log.Fatal("Error creating UDP socket:", err)
	}
	defer unix.Close(serverFd)

	log.Printf("UDP Socket [%v] created\n", serverFd)

	socketAddress := &unix.SockaddrInet4{
		Port: PORT,
	}
	copy(socketAddress.Addr[:], net.ParseIP("0.0.0.0").To4())

	err = unix.Bind(serverFd, socketAddress)
	if err != nil {
		log.Fatal("Error binding UDP socket:", err)
	}
	log.Printf("UDP Socket binded to port %v\n", socketAddress.Port)

	listen()
}

func listen() {
	buffer := make([]byte, BUFFER_SIZE)
	for {
		n, addr, err := unix.Recvfrom(serverFd, buffer, 0)
		if err != nil {
			log.Println("Error reading from UDP socket:", err)
			continue
		}

		handleClient(addr.(*unix.SockaddrInet4), n, buffer)
	}
}

func handleClient(clientAddr *unix.SockaddrInet4, n int, buffer []byte) {
	client := getClientName(clientAddr)
	log.Printf("%s > %s\n", client, string(buffer[:n]))

	blocks, err := handleCommand(string(buffer[:n]))
	if errors.Is(err, FILE_ERROR) {
		block := make([]byte, 4)
		binary.BigEndian.PutUint32(block[0:4], 0xFFFF)

		unix.Sendto(serverFd, block, 0, &unix.SockaddrInet4{Port: clientAddr.Port})
		return
	}
	if err != nil {
		log.Printf("Error handling client %s: %v\n", client, err)
		return
	}

	for _, block := range blocks {
		err = unix.Sendto(serverFd, block, 0, &unix.SockaddrInet4{Port: clientAddr.Port})
		if err != nil {
			log.Printf("Error writing to %s: %v\n", client, err)
			return
		}
	}
}

func handleCommand(command string) ([][]byte, error) {
	parts := strings.Split(strings.Trim(command, " "), " ")
	if len(parts) == 0 {
		return nil, COMMAND_ERROR
	}

	switch parts[0] {
	case "GET":
		return handleGet(parts)
	case "RECOVER":
		return handleRecover(parts)
	default:
		return nil, COMMAND_ERROR
	}
}
