package main

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"

	"golang.org/x/sys/unix"
)

func createMetadataByteArray(file *os.File) ([]byte, error) {
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := uint32(fileInfo.Size())

	md5Sum, err := calculateFileMD5(file)
	if err != nil {
		return nil, err
	}

	data := make([]byte, BUFFER_SIZE)

	binary.BigEndian.PutUint32(data[0:4], 0)
	binary.BigEndian.PutUint32(data[4:8], fileSize)

	copy(data[8:24], md5Sum)

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
