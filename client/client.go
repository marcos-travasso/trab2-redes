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
	"net"
	"os"
	"sort"
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

var files map[string]*TransferingFile
var clientFd int
var serverAddr unix.SockaddrInet4

func main() {
	//setupConnection()
	var err error
	clientFd, err = unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		log.Fatal("Error creating UDP socket:", err)
	}
	defer unix.Close(clientFd)

	log.Printf("UDP Socket [%v] created\n", clientFd)

	serverAddr = unix.SockaddrInet4{
		Port: PORT,
		Addr: [4]byte{127, 0, 0, 1},
	}
	copy(serverAddr.Addr[:], net.ParseIP(IP).To4())

	files = make(map[string]*TransferingFile)

	message := ""
	fmt.Println("-------- UDP SERVER --------")
	fmt.Println("Comandos:")
	fmt.Println("GET <nome do arquivo>: para receber o arquivo. Ex: GET banana.gif")
	fmt.Println("DISCARD <nome do arquivo> <número dos blocos>: para remover blocos recebidos do arquivo. Ex: DISCARD banana.gif 1 5 6")
	fmt.Println("RECOVER <nome do arquivo>: para encontrar os blocos faltantes. Ex: RECOVER banana.gif")
	fmt.Println("BUILD <nome do arquivo>: para juntar os bytes do arquivo. Ex: BUILD banana.gif")
	fmt.Println("SHOW: para mostrar o estado dos arquivos recebidos")
	fmt.Println("----------------------------")

	reader := bufio.NewReader(os.Stdin)
	for string(message) != "QUIT" {
		fmt.Printf("> ")
		message, _ = reader.ReadString('\n')
		message = strings.ReplaceAll(message, "\n", "")

		parts := strings.Split(message, " ")
		switch parts[0] {
		case "GET", "RECOVER":
			sendMessage(message)
		case "BUILD":
			buildFile(parts)
		case "SHOW":
			showFiles()
		case "DISCARD":
			discardBlocks(parts)
		}
	}
}

func sendMessage(message string) {
	parts := strings.Split(message, " ")
	transferingFile, exists := files[parts[1]]
	if !exists && parts[0] == "RECOVER" {
		log.Println("Error: no file to recover")
		return
	}

	if parts[0] == "RECOVER" {
		message += " " + getMissingBlocks(transferingFile)
	}

	err := unix.Sendto(clientFd, []byte(message), 0, &serverAddr)
	if err != nil {
		log.Fatal("Error sending message to server:", err)
	}

	if !exists {
		transferingFile = &TransferingFile{FileName: parts[1], ExpectedBlocks: 65535, ReceivedBlocks: make(map[uint32][]byte), Built: false}
		files[parts[1]] = transferingFile
	} else {
		transferingFile.ExpectedBlocks = transferingFile.TotalBlocks
	}

	handleReceivedBlocks(transferingFile)
}

func handleReceivedBlocks(transferingFile *TransferingFile) {
	for len(transferingFile.ReceivedBlocks) != transferingFile.ExpectedBlocks {
		buffer := make([]byte, BUFFER_SIZE)
		n, _, err := unix.Recvfrom(clientFd, buffer, 0)
		if err != nil {
			log.Fatal("Error receiving response from server:", err)
		}

		if binary.BigEndian.Uint32(buffer[:4]) == 0 {
			readMetadataByteArray(buffer, transferingFile)
			continue
		}

		handleBlock(buffer[:n], transferingFile)
	}
	log.Printf("Blocks transfered successfully!")
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
			continue
		}

		_, err := f.Write(block[4:])
		if err != nil {
			f.Close()
			log.Fatalln("Error writing file:", err)
		}
	}

	transferingFile.Built = true

	if !verifyMD5(transferingFile) {
		log.Printf("Built file may be corrupted")
		return
	}

	log.Println("File built successfully!")
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
}

func readMetadataByteArray(data []byte, transferingFile *TransferingFile) {
	fileSize := binary.BigEndian.Uint32(data[4:8])
	totalBlocks := binary.BigEndian.Uint32(data[24:28])

	md5Hash := make([]byte, 16)
	copy(md5Hash, data[8:24])

	transferingFile.FileSize = fileSize
	transferingFile.MD5Hash = md5Hash
	transferingFile.TotalBlocks = int(totalBlocks) - 1
	transferingFile.ExpectedBlocks = int(totalBlocks) - 1
}

func showFiles() {
	if len(files) == 0 {
		log.Println("No files to show")
	}

	for _, file := range files {
		printFileInfo(file)
	}
}

func printFileInfo(file *TransferingFile) {
	fmt.Println("---------------------------------------------")
	fmt.Printf("File: %s\n", file.FileName)
	fmt.Printf("File size: %d\n", file.FileSize)
	fmt.Printf("Received blocks: %d\n", len(file.ReceivedBlocks))
	fmt.Printf("Total blocks: %d\n", file.TotalBlocks)
	if file.TotalBlocks-len(file.ReceivedBlocks) != 0 {
		fmt.Printf("\tMissing Blocks: %s\n", getMissingBlocks(file))
	}
	fmt.Printf("MD5: %x\n", file.MD5Hash)
	fmt.Printf("Built: %t\n", file.Built)
	fmt.Println("---------------------------------------------")
}

func getMissingBlocks(transferingFile *TransferingFile) string {
	var missing []uint32
	var blockNumbers []uint32

	for block := range transferingFile.ReceivedBlocks {
		blockNumbers = append(blockNumbers, block)
	}

	sort.Slice(blockNumbers, func(i, j int) bool { return blockNumbers[i] < blockNumbers[j] })

	for i := 1; i < len(blockNumbers); i++ {
		if blockNumbers[i]-blockNumbers[i-1] > 1 {
			for missingBlock := blockNumbers[i-1] + 1; missingBlock < blockNumbers[i]; missingBlock++ {
				missing = append(missing, missingBlock)
			}
		}
	}

	result := ""
	for _, r := range missing {
		result += fmt.Sprintf("%d ", r)
	}

	return result
}

func getFileName(data []byte) string {
	return string(data[28 : getEOF(data[28:])+28])
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

	serverAddr = unix.SockaddrInet4{
		Port: PORT,
		Addr: [4]byte{127, 0, 0, 1},
	}
	copy(serverAddr.Addr[:], net.ParseIP(IP).To4())
}
