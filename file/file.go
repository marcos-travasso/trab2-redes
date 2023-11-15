package main

import (
	"io"
	"os"
)

func main() {
	sourceFile := "../server/banana.gif"
	newFile := "./banana-r.gif"

	// Open the source file for reading
	source, err := os.Open(sourceFile)
	if err != nil {
		panic(err)
	}
	defer source.Close()

	// Open the new file for writing
	outputFile, err := os.Create(newFile)
	if err != nil {
		panic(err)
	}
	defer outputFile.Close()
	fi, _ := source.Stat()
	tot := 0
	buffer := make([]byte, 10000)

	for tot != int(fi.Size()) {
		// Read 16 bytes from the source file
		n, err := source.Read(buffer)
		tot += n
		if err != nil && err != io.EOF {
			panic(err)
		}

		// Write the read bytes to the new file
		_, err = outputFile.Write(buffer[:n])
		if err != nil {
			panic(err)
		}
	}

}
