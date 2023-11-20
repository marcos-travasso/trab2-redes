package main

import (
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

	return joinBytes(metadata, data), nil
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
	toRecover := make([][]byte, len(parts)-2)

	for _, blockN := range parts[2:] {
		num, _ := strconv.Atoi(blockN)
		num--

		if num < len(dataBlocks) {
			toRecover = append(toRecover, dataBlocks[num])
		}
	}

	return toRecover, nil
}
