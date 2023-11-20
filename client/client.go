package main

import (
	"bufio"
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
	"strconv"
	"strings"
)

type TransferingFile struct {
	FileSize       uint32
	MD5Hash        []byte
	FileName       string
	TotalBlocks    int
	ExpectedBlocks int
	ReceivedBlocks map[uint32][]byte
	Built          bool
}

const BUFFER_SIZE = 2048
const IP = "127.0.0.1"
const PORT = 8080

var files map[string]TransferingFile
var clientFd int
var serverAddr *unix.SockaddrInet4

func main() {
	setupConnection()
	files = make(map[string]TransferingFile)

	message := ""
	fmt.Println("-------- UDP SERVER --------")
	fmt.Println("Comandos:")
	fmt.Println("GET <nome do arquivo>: para receber o arquivo. Ex: GET banana.gif")
	fmt.Println("DISCARD <nome do arquivo> <número dos blocos>: para remover blocos recebidos do arquivo. Ex: DISCARD banana.gif 1 5 6")
	fmt.Println("RECOVER <nome do arquivo> <número dos blocos>: para encontrar os blocos faltantes. Ex: RECOVER banana.gif 1 5 6")
	fmt.Println("BUILD <nome do arquivo>: para juntar os bytes do arquivo. Ex: BUILD banana.gif")
	fmt.Println("SHOW: para mostrar o estado dos arquivos recebidos")
	fmt.Println("----------------------------")

	reader := bufio.NewReader(os.Stdin)
	for string(message) != "QUIT" {
		// Message to send
		//message := []byte("GET hello2.txt")
		//message := []byte("GET banana.gif")
		//message = []byte("RECOVER banana.gif 1 2 3 4")
		message, _ = reader.ReadString('\n')
		message = strings.ReplaceAll(message, "\n", "")

		parts := strings.Split(message, " ")
		switch parts[0] {
		case "GET", "RECOVER":
			sendMessage([]byte(message))
		case "BUILD":
			buildFile(parts)
		case "SHOW":
			showFiles()
		case "DISCARD":
			discardBlocks(parts)
		}
	}
}

func sendMessage(message []byte) {
	err := unix.Sendto(clientFd, message, 0, serverAddr)
	if err != nil {
		log.Fatal("Error sending message to server:", err)
	}

	parts := strings.Split(string(message), " ")
	transferingFile, exists := files[parts[1]]
	if !exists && parts[0] == "RECOVER" {
		log.Println("Error: no file to recover")
	}

	if !exists {
		transferingFile = TransferingFile{ExpectedBlocks: 65535, ReceivedBlocks: make(map[uint32][]byte), Built: false}
		files[parts[0]] = transferingFile
	}

	if parts[0] == "GET" {
		handleReceivedBlocks(&transferingFile)
		return
	}

	if parts[0] == "RECOVER" {
		transferingFile.ExpectedBlocks = len(parts) - 2
		handleReceivedBlocks(&transferingFile)
		return
	}
}

func handleReceivedBlocks(transferingFile *TransferingFile) {
	for len(transferingFile.ReceivedBlocks) != transferingFile.TotalBlocks {
		buffer := make([]byte, BUFFER_SIZE)
		n, _, err := unix.Recvfrom(clientFd, buffer, 0)
		if err != nil {
			log.Fatal("Error receiving response from server:", err)
		}

		if n < 4 {
			continue
		}

		if binary.BigEndian.Uint32(buffer[:4]) == 0 {
			readMetadataByteArray(buffer, transferingFile)
			continue
		}

		handleBlock(buffer[:n], transferingFile)
	}

	if !verifyMD5(transferingFile) {
		log.Printf("Received file may be corrupted")
		return
	}

	log.Printf("File transfered successfully!")
}

func verifyMD5(transferingFile *TransferingFile) bool {
	f, err := os.Open(transferingFile.FileName)
	if err != nil {
		return false
	}
	defer f.Close()

	fileMD5, err := calculateFileMD5(f)
	if err != nil {
		return false
	}

	return hex.EncodeToString(fileMD5) == hex.EncodeToString(transferingFile.MD5Hash)
}

func buildFile(parts []string) {
	transferingFile, exists := files[parts[1]]
	if !exists {
		log.Println("File not found to build")
		return
	}

	f, err := os.Create("./" + transferingFile.FileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	for i := 1; i <= len(transferingFile.ReceivedBlocks); i++ {
		block, exists := transferingFile.ReceivedBlocks[uint32(i)]
		if !exists {
			log.Printf("Error: block %d not found\n", i)
		}

		_, err := f.Write(block[4:])
		if err != nil {
			f.Close()
			log.Fatalln("Error writing file:", err)
		}
	}

	transferingFile.Built = true
}

func discardBlocks(parts []string) {
	transferingFile, exists := files[parts[1]]
	if !exists {
		log.Println("File not found to discard")
		return
	}

	discarded := 0
	for _, blockN := range parts[2:] {
		num, _ := strconv.Atoi(blockN)
		_, exists = transferingFile.ReceivedBlocks[uint32(num)]
		if exists {
			delete(transferingFile.ReceivedBlocks, uint32(num))
			discarded++
		}
	}

	log.Printf("Discarded %x blocks\n", discarded)
}

func handleBlock(buffer []byte, transferingFile *TransferingFile) {
	position := binary.BigEndian.Uint32(buffer[:4])
	transferingFile.ReceivedBlocks[position] = make([]byte, len(buffer))
	copy(transferingFile.ReceivedBlocks[position], buffer)
	log.Printf("DEBUG %d\tlen %d\tmd5: %s\n", position, len(transferingFile.ReceivedBlocks[position]), calculateMD5(transferingFile.ReceivedBlocks[position]))
}

func readMetadataByteArray(data []byte, transferingFile *TransferingFile) {
	fileSize := binary.BigEndian.Uint32(data[4:8])

	md5Hash := make([]byte, 16)
	copy(md5Hash, data[8:24])

	fileName := getFileName(data)

	transferingFile.FileSize = fileSize
	transferingFile.MD5Hash = md5Hash
	transferingFile.FileName = fileName
	transferingFile.TotalBlocks = int(math.Ceil(float64(fileSize) / float64(BUFFER_SIZE-2)))
	transferingFile.ExpectedBlocks = transferingFile.TotalBlocks

	fmt.Println("-------------------------------------")
	fmt.Printf("File Name: %s\n", transferingFile.FileName)
	fmt.Printf("File Size: %d\n", transferingFile.FileSize)
	fmt.Printf("MD5 Hash: %x\n", transferingFile.MD5Hash)
	fmt.Printf("Total Blocks: %x\n", transferingFile.TotalBlocks)
	fmt.Println("-------------------------------------")
}

func showFiles() {
	if len(files) == 0 {
		log.Println("No files to show")
	}

	for _, file := range files {
		fmt.Println("---------------------------")
		fmt.Printf("Arquivo: %s\n", file.FileName)
		fmt.Printf("\tTamanho do arquivo: %d\n", file.FileSize)
		fmt.Printf("\tBlocos recebidos: %d\n", len(file.ReceivedBlocks))
		fmt.Printf("\tBlocos totais: %d\n", file.TotalBlocks)
		if file.TotalBlocks-len(file.ReceivedBlocks) != 0 {
			fmt.Printf("\t\tBlocos faltantes: %s\n", getRemainingBlocks(&file))
		}
		fmt.Printf("\tConstruído: %t", file.Built)
		fmt.Println("---------------------------")
	}
}

func getRemainingBlocks(transferingFile *TransferingFile) string {
	remaining := make([]uint32, 0)
	last := uint32(0)
	for block, _ := range transferingFile.ReceivedBlocks {
		if block-last > 1 {
			remaining = append(remaining, block)
		}
		last = block
	}

	result := ""
	for _, r := range remaining {
		result += fmt.Sprintf("%d ", r)
	}

	return result
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

func calculateMD5(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

func setupConnection() {
	var err error
	clientFd, err = unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		log.Fatal("Error creating UDP socket:", err)
	}
	defer unix.Close(clientFd)

	log.Printf("UDP Socket [%v] created\n", clientFd)

	serverAddr = &unix.SockaddrInet4{
		Port: PORT,
	}
	copy(serverAddr.Addr[:], net.ParseIP(IP).To4())
}
