# Video
https://youtu.be/jy5ZVzhsBd4

# Como executar

Há duas opções para executar o trabalho:

## Usando Linux/WSL

Neste caso é preciso que a máquina possua o [Go 1.12.1 instalado](https://go.dev/doc/install)

Após isso, entre no diretório do servidor e rode:

```shell
go run server.go
```

E para entrar como cliente, abra um outro (ou múltiplos) terminal, entre no diretório do cliente e rode:

```shell
go run client.go
```

O STDIN do terminal com o cliente estará disponível para enviar mensagens ao apertar ENTER.

## Usando Docker

Como a biblioteca que utilizei em Go é a `unix`, não há uma compatibilidade para rodar o projeto diretamente de um terminal no Windows, por isso, caso precise rodar por lá, é possível fazer usando o WSL como na opção anterior, ou através de containers:

Entre no diretório do servidor e construa a imagem do container com:

```shell
docker build -t redes-server .
```

E inicie o servidor subindo um container:

```shell
docker run --rm -p 8080:8080 redes-server
```

Para usar o cliente, construa a imagem estando no diretório do cliente:
```shell
docker build -t redes-client .
```

E inicie um cliente com:
```shell
docker run --rm -i --network host redes-client
```

## HTTP
Entre no endereço `http://localhost:8080` para acessar a "página" em HTML.

### Acessando de outro dispositivo
Nesse caso, garanta que a porta está sendo exposta na rede e que os dispositivos estão conectados na mesma rede, após isso, encontre o IP da máquina servidor e acesse pelo cliente `http://<ip do servidor>:<porta do servidor>`