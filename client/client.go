package main

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	llq "github.com/emirpasic/gods/queues/linkedlistqueue"
	"golang.org/x/sys/unix"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

var clientFd int
var server = &unix.SockaddrInet4{
	Port: 8080,
	Addr: [4]byte(net.ParseIP("127.0.0.1")), //Colocar aqui o endereço local do servidor
}

var transfering = false
var messagesCh chan []byte
var fileTransferQueue *llq.Queue

func main() {
	var err error
	//Mesmos parâmetros do socket do servidor
	clientFd, err = unix.Socket(unix.AF_INET, unix.SOCK_STREAM, unix.IPPROTO_IP)
	if err != nil {
		log.Fatalln("Error creating socket:", err)
	}
	defer unix.Close(clientFd)
	log.Printf("Client socket [%v] created\n", clientFd)

	//Conexão com o socket do servidor
	err = unix.Connect(clientFd, server)
	if err != nil {
		log.Fatalln("Error connecting sockets:", err)
	}
	log.Printf("Server socket [%v] connected\n", server.Port)

	messagesCh = make(chan []byte)
	fileTransferQueue = llq.New()

	go handleMessage()
	//Reader do teclado para enviar as mensagens para o servidor
	reader := bufio.NewReader(os.Stdin)
	for {
		//Processamento do que foi escrito e envio para o servidor
		msg, _ := reader.ReadString('\n')
		msg = strings.ReplaceAll(msg, "\n", "")
		_, err = unix.Write(clientFd, []byte(msg))
		if err != nil {
			log.Fatalln("Error writing:", err)
		} else if strings.Contains(strings.ToLower(msg), "arquivo") {
			//A forma como as mensagens serão recebidas para trocar arquivo segue um protocolo,
			//por isso é bloqueado o envio de novas mensagens até terminar a comunicação
			transfering = true
			handleTransferFile()
			transfering = false
		}
	}
}

func handleMessage() {
	go func() {
		for msg := range messagesCh {
			//Caso de transferência vai ser tratado pelo protocolo de transferência, então repassa pra uma fila que vai cuidar dos bytes recebidos
			if transfering {
				fileTransferQueue.Enqueue(msg)
				continue
			}

			//Caso em que o desligamento foi feito com sucesso
			if string(msg) == "Closing connection" {
				log.Fatalln("Closed connection")
			}

			//Mensagem recebida do servidor
			log.Println(string(msg))
		}
	}()

	//Alocação de um buffer para receber a resposta do servidor
	buffer := make([]byte, 1024*1024*1024)
	for {
		//Leitura da resposta do servidor
		n, err := unix.Read(clientFd, buffer)
		if err != nil {
			log.Fatalln("Error reading:", err)
		}

		//Caso padrão quando o servidor retorna uma mensagem (no caso, a mesma mensagem enviada)
		if n != 0 {
			messagesCh <- buffer[:n]
		}
	}
}

func handleTransferFile() {
	var err error

	//Espera para receber os metadados do arquivo a receber
	receivedMsg := waitForMessage()

	if string(receivedMsg) == "File not found" {
		log.Printf("File not found by the server")
		return
	}

	//Análise dos metadados recebidos
	fileMetadata := strings.Split(string(receivedMsg), "|")
	log.Printf("File metadata: %s\n", fileMetadata)

	//Confirmação do recebimento para o servidor
	_, err = unix.Write(clientFd, []byte("ack"))
	if err != nil {
		log.Fatalln("Error writing ack:", err)
	}

	//Criação do arquivo a ser recebido com o tamanho especificado nos metadados
	size, _ := strconv.ParseInt(fileMetadata[1], 10, 64)
	f, err := os.Create("./" + fileMetadata[0])
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	log.Printf("Waiting for file with size %s\n", humanizeBytes(size))

	//Esse received serve para esperar todos os bytes chegarem, então cada vez que o servidor envia uma leva de dados,
	//o received é incrementado pelo tamanho até chegar no tamanho esperado para o arquivo (o que veio nos metadados)
	received := 0
	for int64(received) < size {
		//Espera pelo arquivo
		receivedBytes := waitForMessage()
		log.Printf("Received %s writing to disk\n", humanizeBytes(int64(len(receivedBytes))))

		//Escreve o arquivo no disco
		n, err := f.Write(receivedBytes)
		received += n
		if err != nil {
			f.Close()
			log.Fatalln("Error writing file:", err)
		}
		log.Printf("Written %s/%s to disk\n", humanizeBytes(int64(received)), humanizeBytes(size))
	}

	log.Println("File written, checking sums")

	//Abre o arquivo recebido para comparar o hash
	receivedData := make([]byte, size)
	f.Seek(0, 0)
	_, err = f.Read(receivedData)
	if err != nil {
		log.Fatal(err)
	}

	sum := sha1.Sum(receivedData)
	if fmt.Sprintf("%x", sum) != fileMetadata[2] {
		log.Printf("Sums doesn't match!\n\tReceived: %x\n\tExpected: %s\n\n", sum, fileMetadata[2])
		unix.Write(clientFd, []byte("nack"))
		return
	}

	unix.Write(clientFd, []byte("ack"))
	log.Println("Sums checked, file received!")
}

func humanizeBytes(b int64) string {
	bytes := float64(b)
	suffixes := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}

	i := 0
	for bytes >= 1024 && i < len(suffixes)-1 {
		bytes /= 1024
		i++
	}

	return fmt.Sprintf("%.2f%s", bytes, suffixes[i])
}

func waitForMessage() []byte {
	var i interface{}
	ok := false

	for !ok {
		i, ok = fileTransferQueue.Dequeue()
	}

	return i.([]byte)
}
