package main

import (
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

var PORT = 8080
var serverFd int
var BUFFER_SIZE = 2048

var COMMAND_ERROR = errors.New("Command error")
var FILE_ERROR = errors.New("File error")

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

	responses, err := handleCommand(string(buffer[:n]))
	if err != nil {
		log.Printf("Error handling client %s: %v\n", client, err)
		return
	}
	log.Printf("DEBUG sending %d blocks to %s\n", len(responses), client)

	for _, response := range responses {
		log.Printf("sending: %x (len %d)\n", response, len(response))
		err = unix.Sendto(serverFd, response, 0, &unix.SockaddrInet4{Port: clientAddr.Port})
		if err != nil {
			log.Printf("Error writing to %s: %v\n", client, err)
			return
		}
	}
}

func handleCommand(command string) ([][]byte, error) {
	parts := strings.Split(command, " ")
	if len(parts) < 2 || parts[0] != "GET" {
		return nil, COMMAND_ERROR
	}

	if len(parts) > 3 {
		return handleSpecificGet(command)
	}

	return handleGet(parts)
}

func handleGet(parts []string) ([][]byte, error) {
	file, err := os.Open("./" + parts[1])
	s, err2 := file.Stat()
	if err != nil || err2 != nil || s.IsDir() {
		return nil, FILE_ERROR
	}
	defer file.Close()

	metadata, err := createMetadataByteArray(file)
	if err != nil {
		return nil, err
	}

	data, err := createDataByteArray(file, s.Size())
	if err != nil {
		return nil, err
	}

	return joinBytes(metadata, data), nil
}

func handleSpecificGet(command string) ([][]byte, error) {
	return nil, nil
}

func createMetadataByteArray(file *os.File) ([]byte, error) {
	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := uint32(fileInfo.Size())

	// Get MD5 hash of the file content
	md5Sum, err := calculateMD5(file)
	if err != nil {
		return nil, err
	}

	// Create byte array
	data := make([]byte, BUFFER_SIZE)

	binary.BigEndian.PutUint32(data[0:4], 0)
	binary.BigEndian.PutUint32(data[4:8], fileSize)

	// Set next 128 bits to file MD5 hash
	copy(data[8:24], md5Sum)

	// Set the rest with the filename, padding with null bytes if needed
	filenameBytes := []byte(fileInfo.Name())
	copy(data[24:], filenameBytes)

	return data[:24+len(filenameBytes)], nil
}

func createDataByteArray(file *os.File, size int64) ([]byte, error) {
	file.Seek(0, 0)

	data := make([]byte, size)
	_, err := file.Read(data)
	return data, err
}

func joinBytes(metadata, data []byte) [][]byte {
	dataSize := len(data)
	totalDataBlocks := int(math.Ceil(float64(dataSize)/2046)) + 1
	blocks := make([][]byte, totalDataBlocks)
	log.Printf("size: %d, totaldata %d, blocks %d\n", dataSize, totalDataBlocks, blocks)

	blocks[0] = metadata
	log.Printf("METADATA %x\n", blocks[0])
	i := 1
	for dataSize >= BUFFER_SIZE-2 {
		log.Printf("writing inside loop")
		blocks[i] = make([]byte, BUFFER_SIZE)
		binary.BigEndian.PutUint32(blocks[i][:2], uint32(i))
		copy(blocks[i][2:], data[(i-1)*(BUFFER_SIZE-2):(i-1)*(BUFFER_SIZE-2)+BUFFER_SIZE-2])
		i++
		dataSize -= BUFFER_SIZE - 2
	}

	if dataSize > 0 {
		log.Printf("writing in if %d with i %d\n", dataSize, i)
		blocks[i] = make([]byte, dataSize+4)
		log.Printf("create datasize (%d ou %d) %d\n", i, uint32(i), dataSize+4)
		binary.BigEndian.PutUint32(blocks[i][:4], uint32(i))
		copy(blocks[i][4:], data[(i-1)*(BUFFER_SIZE-2):])
	}

	return blocks
}

func calculateMD5(file *os.File) ([]byte, error) {
	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, err
	}
	log.Printf("DEBUG MD5: %x\n", hasher.Sum(nil))
	return hasher.Sum(nil), nil
}

func getClientName(c *unix.SockaddrInet4) string {
	return fmt.Sprintf("%d.%d.%d.%d:%d", c.Addr[0], c.Addr[1], c.Addr[2], c.Addr[3], c.Port)
}
