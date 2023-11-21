package main

import (
	"encoding/binary"
	"os"
	"strconv"
)

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

	result, err := joinBytes(metadata, data), nil
	if err != nil {
		return nil, err
	}

	binary.BigEndian.PutUint32(result[0][24:28], uint32(len(result)))

	return result, nil
}

func handleRecover(parts []string) ([][]byte, error) {
	if len(parts) == 2 {
		return handleGet(parts)
	} else if len(parts) < 2 {
		return nil, COMMAND_ERROR
	}

	file, err := os.Open("./" + parts[1])
	s, err2 := file.Stat()
	if err != nil || err2 != nil || s.IsDir() {
		return nil, FILE_ERROR
	}
	defer file.Close()

	data, err := createDataByteArray(file, s.Size())
	if err != nil {
		return nil, err
	}

	dataBlocks := divideDataInBlocks(data)
	toRecover := make([][]byte, len(parts)-2, len(parts)-2)
	i := 0

	for _, blockN := range parts[2:] {
		num, _ := strconv.Atoi(blockN)
		num--

		if num < len(dataBlocks) {
			toRecover[i] = dataBlocks[num]
			i++
		}
	}

	return toRecover, nil
}
