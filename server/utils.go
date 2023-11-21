package main

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"

	"golang.org/x/sys/unix"
)

func createMetadataByteArray(file *os.File) ([]byte, error) {
	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := uint32(fileInfo.Size())

	// Get MD5 hash of the file content
	md5Sum, err := calculateFileMD5(file)
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
	copy(data[28:], filenameBytes)

	return data[:28+len(filenameBytes)], nil
}

func createDataByteArray(file *os.File, size int64) ([]byte, error) {
	file.Seek(0, 0)

	data := make([]byte, size)
	_, err := file.Read(data)
	return data, err
}

func joinBytes(metadata, data []byte) [][]byte {
	blocks := divideDataInBlocks(data)

	result := make([][]byte, 1+len(blocks), 1+len(blocks))
	result[0] = metadata

	for i, block := range blocks {
		result[i+1] = block
	}

	return result
}

func divideDataInBlocks(data []byte) [][]byte {
	dataSize := len(data)
	totalDataBlocks := int(math.Ceil(float64(dataSize) / float64(BUFFER_SIZE-2)))
	blocks := make([][]byte, totalDataBlocks)

	i := 0
	for dataSize >= BUFFER_SIZE-2 {
		blocks[i] = make([]byte, BUFFER_SIZE)
		binary.BigEndian.PutUint32(blocks[i][:4], uint32(i+1))
		copy(blocks[i][4:], data[i*(BUFFER_SIZE-4):i*(BUFFER_SIZE-2)+BUFFER_SIZE-4])
		//log.Printf("LENS %d e %d\n", len(blocks[i][4:]), len(data[(i-1)*(BUFFER_SIZE-2):(i-1)*(BUFFER_SIZE-2)+BUFFER_SIZE-2]))
		//log.Printf("Segmento %d\t %d-%d \t %s\n", i, (i-1)*(BUFFER_SIZE-2), (i-1)*(BUFFER_SIZE-2)+BUFFER_SIZE-2, string(blocks[i][4:]))
		//log.Printf("removendo: %d dev: %d\n", BUFFER_SIZE-2, len(blocks[i][4:]))
		dataSize -= len(blocks[i][4:])
		i++
	}

	if dataSize > 0 {
		blocks[i] = make([]byte, dataSize+4)
		binary.BigEndian.PutUint32(blocks[i][:4], uint32(i+1))
		copy(blocks[i][4:], data[i*(BUFFER_SIZE-4):])
	}

	return blocks
}

func calculateMD5(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

func calculateFileMD5(file *os.File) ([]byte, error) {
	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, err
	}
	return hasher.Sum(nil), nil
}

func getClientName(c *unix.SockaddrInet4) string {
	return fmt.Sprintf("%d.%d.%d.%d:%d", c.Addr[0], c.Addr[1], c.Addr[2], c.Addr[3], c.Port)
}
