package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"

	"golang.org/x/sys/unix"
)

const BUFFER_SIZE = 2048

type Metadata struct {
	FileSize uint32
	MD5Hash  []byte
	FileName string
}

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
	message := []byte("GET hello.txt")

	// Send the message to the server
	err = unix.Sendto(clientFd, message, 0, serverAddr)
	if err != nil {
		log.Fatal("Error sending message to server:", err)
	}

	log.Printf("Message sent to the server: %s\n", string(message))

	received := make(map[uint32][]byte)
	totalBlocks := 65535
	for len(received) != totalBlocks {

		// Receive the response from the server
		buffer := make([]byte, BUFFER_SIZE)
		n, _, err := unix.Recvfrom(clientFd, buffer, 0)
		if err != nil {
			log.Fatal("Error receiving response from server:", err)
		}

		if n < 4 {
			continue
		}

		if binary.BigEndian.Uint32(buffer[:4]) == 0 {
			totalBlocks = handleMetadata(buffer)
			log.Printf("DEBUG Total de blocos: %d\n", totalBlocks)
			continue
		}

		handleBlock(buffer[:n], received)
	}
}

func handleBlock(buffer []byte, received map[uint32][]byte) {
	position := binary.BigEndian.Uint32(buffer[:4])
	received[position] = make([]byte, len(buffer))
	copy(received[position], buffer)
	log.Printf("DEBUG Bloco %d recebeu: %x\n", position, received[position])
}

func handleMetadata(buffer []byte) int {
	metadata, err := readMetadataByteArray(buffer)
	if err != nil {
		log.Fatal("Error reading metadata:", err)
	}

	fmt.Println("-------------------------------------")
	fmt.Printf("File Size: %d\n", metadata.FileSize)
	fmt.Printf("MD5 Hash: %x\n", metadata.MD5Hash)
	fmt.Printf("File Name: %s\n", metadata.FileName)
	fmt.Println("-------------------------------------")
	return int(math.Ceil(float64(metadata.FileSize) / 2046))
}

func readMetadataByteArray(data []byte) (*Metadata, error) {
	fileSize := binary.BigEndian.Uint32(data[4:8])

	md5Hash := make([]byte, 16)
	copy(md5Hash, data[8:24])

	// Read file name
	fileName := string(data[24:])

	return &Metadata{
		FileSize: fileSize,
		MD5Hash:  md5Hash,
		FileName: fileName,
	}, nil
}
