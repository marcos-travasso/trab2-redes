package main

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"log"
	"math"
	"net"
	"os"
)

const BUFFER_SIZE = 2048

type Metadata struct {
	FileSize    uint32
	MD5Hash     []byte
	FileName    string
	TotalBlocks int
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
	//message := []byte("GET hello2.txt")
	message := []byte("GET banana.gif")

	// Send the message to the server
	err = unix.Sendto(clientFd, message, 0, serverAddr)
	if err != nil {
		log.Fatal("Error sending message to server:", err)
	}

	log.Printf("Message sent to the server: %s\n", string(message))

	receivedBlocks := make(map[uint32][]byte)
	metadata := Metadata{TotalBlocks: 65535}
	for len(receivedBlocks) != metadata.TotalBlocks {
		buffer := make([]byte, BUFFER_SIZE)
		n, _, err := unix.Recvfrom(clientFd, buffer, 0)
		if err != nil {
			log.Fatal("Error receiving response from server:", err)
		}

		if n < 4 {
			continue
		}

		if binary.BigEndian.Uint32(buffer[:4]) == 0 {
			metadata = handleMetadata(buffer)
			continue
		}

		handleBlock(buffer[:n], receivedBlocks)
	}

	buildFile(metadata, receivedBlocks)

	if !verifyMD5(metadata) {
		log.Fatal("Received file may be corrupted")
	}

	log.Printf("File transfered successfully!")
}

func verifyMD5(metadata Metadata) bool {
	f, err := os.Open(metadata.FileName)
	if err != nil {
		return false
	}
	defer f.Close()

	fileMD5, err := calculateFileMD5(f)
	if err != nil {
		return false
	}

	return hex.EncodeToString(fileMD5) == hex.EncodeToString(metadata.MD5Hash)
}

func buildFile(metadata Metadata, blocks map[uint32][]byte) {
	f, err := os.Create("./" + metadata.FileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	for i := 1; i <= len(blocks); i++ {

		block, exists := blocks[uint32(i)]
		if !exists {
			log.Printf("Error: block %d not found\n", i)
		}

		_, err := f.Write(block[4:])
		if err != nil {
			f.Close()
			log.Fatalln("Error writing file:", err)
		}
	}
}

func handleBlock(buffer []byte, received map[uint32][]byte) {
	position := binary.BigEndian.Uint32(buffer[:4])
	received[position] = make([]byte, len(buffer))
	copy(received[position], buffer)
	//log.Printf("DEBUG %d\tlen %d\tmd5: %s\n", position, len(received[position]), calculateMD5(received[position]))
}

func handleMetadata(buffer []byte) Metadata {
	metadata, err := readMetadataByteArray(buffer)
	if err != nil {
		log.Fatal("Error reading metadata:", err)
	}

	fmt.Println("-------------------------------------")
	fmt.Printf("File Size: %d\n", metadata.FileSize)
	fmt.Printf("MD5 Hash: %x\n", metadata.MD5Hash)
	fmt.Printf("File Name: %s\n", metadata.FileName)
	fmt.Println("-------------------------------------")
	return metadata
}

func readMetadataByteArray(data []byte) (Metadata, error) {
	fileSize := binary.BigEndian.Uint32(data[4:8])

	md5Hash := make([]byte, 16)
	copy(md5Hash, data[8:24])

	fileName := getFileName(data)

	return Metadata{
		FileSize:    fileSize,
		MD5Hash:     md5Hash,
		FileName:    fileName,
		TotalBlocks: int(math.Ceil(float64(fileSize) / float64(BUFFER_SIZE-2))),
	}, nil
}

func getFileName(data []byte) string {
	return string(data[24 : getEOF(data[24:])+24])
}

func getEOF(data []byte) int {
	for i, b := range data {
		if b == 0 {
			return i
		}
	}
	return len(data)
}

func calculateFileMD5(file *os.File) ([]byte, error) {
	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, err
	}
	log.Printf("DEBUG MD5: %x\n", hasher.Sum(nil))
	return hasher.Sum(nil), nil
}
