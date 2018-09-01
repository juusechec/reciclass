package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	//"time"
	"crypto/aes"
	"crypto/cipher"
	"errors"
  "encoding/json"
  "strconv"
)

// Example SSE server in Golang.
// $ go run sse.go
// based on https://gist.github.com/ismasan/3fb75381cd2deb6bfa9c

type Broker struct {

	// Events are pushed to this channel by the main events-gathering routine
	Notifier chan []byte

	// New client connections
	newClients chan chan []byte

	// Closed client connections
	closingClients chan chan []byte

	// Client connections registry
	clients map[chan []byte]bool
}

type Isla struct {
  Id string `json:"id"` // Identificador de la Isla
  St int `json:"st"` // Estado de la Isla
}

func NewServer() (broker *Broker) {
	// Instantiate a broker
	broker = &Broker{
		Notifier:       make(chan []byte, 1),
		newClients:     make(chan chan []byte),
		closingClients: make(chan chan []byte),
		clients:        make(map[chan []byte]bool),
	}

	// Set it running - listening and broadcasting events
	go broker.listen()

	return
}

func string2Byte(hex byte) byte {
	switch hex {
	case '0':
		return 0
	case '1':
		return 1
	case '2':
		return 2
	case '3':
		return 3
	case '4':
		return 4
	case '5':
		return 5
	case '6':
		return 6
	case '7':
		return 7
	case '8':
		return 8
	case '9':
		return 9
	case 'A':
		return 10
	case 'B':
		return 11
	case 'C':
		return 12
	case 'D':
		return 13
	case 'E':
		return 14
	case 'F':
		return 15
	}
	return 0
}

func HEX2Byte(hex1 byte, hex2 byte) byte {
	var a byte = string2Byte(hex1)
	var b byte = string2Byte(hex2)
	//fmt.Println(a, b)
	a = a << 4
	return a | b
}

func decryptCBC(key []byte, text string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	decodedMsg := []byte(text)
	fmt.Println((decodedMsg))
	if (len(decodedMsg) % aes.BlockSize) != 0 {
		return "", errors.New("blocksize must be multipe of decoded message length")
	}

	//iv := decodedMsg[:aes.BlockSize]
	iv := []byte("abcdefghijklmnop")
	msg := decodedMsg
	fmt.Println(iv)
	cbc := cipher.NewCBCDecrypter(block, iv)
	cbc.CryptBlocks(msg, msg)

	//unpadMsg, err := Unpad(msg)
	unpadMsg := (msg)
	if err != nil {
		return "", err
	}

	return string(unpadMsg), nil
}

func (broker *Broker) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
  log.Println(req.RequestURI)
	u, err := url.Parse(req.RequestURI)
	if err != nil {
		log.Fatal(err)
	}
	// Make sure that the writer supports flushing.
	//
	if u.EscapedPath() == "/dev" {
		//Se trata la petición para obtener el parámetro data
		q := u.Query()
		//log.Println(u, q, u.String(), u.EscapedPath())
		data := q["data"][0]
		fmt.Println("Dato que llega: ", data)
    // //Se escribe sobre todos los otros navegadores
		// broker.Notifier <- []byte(data)
		//Se responde al Microcontrolador con las cabeceras correspondientes
		rw.Header().Set("Content-Type", "text/html; charset=UTF-8")
		rw.Header().Set("Cache-Control", "no-cache")
		rw.Header().Set("Connection", "close")
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		io.WriteString(rw, data+"\n") //Opcional

		//Se evalua el tamaño del mensaje para volverlo a caracteres codificados
		var count int = len(data) / 2
		fmt.Println("Count: ", count, ", Length:", len(data))
		cadena := make([]byte, count)
		for i := 0; i < count; i++ {
			var a int = i * 2
			var b int = a + 1
			// fmt.Println(a, b, data[a], data[b])
			var c byte = HEX2Byte(data[a], data[b])
			// fmt.Println(c)
			// fmt.Println(i)
			cadena[i] = c
		}
		// https://gist.github.com/kkirsche/e28da6754c39d5e7ea10
		// https://gist.github.com/cannium/c167a19030f2a3c6adbb5a5174bea3ff
		// fmt.Println("Cadena: ", cadena)
		cadenaAES256 := string(cadena[:])
		fmt.Println("Dato cifrado: ", cadenaAES256)
		// Se comienza la decodificación

		var key = []byte("omarleonardozamb")
		text, _ := decryptCBC(key, cadenaAES256)

    fmt.Printf("Dato descifrado: %s\n", text)
    // Se lee el texto que llega como una URL se agrega siempre "?"
    u, err = url.Parse("?" + text)
    if err != nil{
      log.Fatal(err)
    }
    //Se trata la petición para obtener el parámetro data
		q = u.Query()
    idIsla := q["id"][0]
    stIsla, _ := strconv.Atoi(q["st"][0]) // Convertir de String a Int
    isla := Isla{Id: idIsla, St: stIsla}

    b, _ := json.Marshal(isla)
    fmt.Println(string(b))
		fmt.Println()
    //Se escribe sobre todos los otros navegadores
		broker.Notifier <- []byte(string(b))
	} else {
		flusher, ok := rw.(http.Flusher)

		if !ok {
			http.Error(rw, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "text/event-stream")
		rw.Header().Set("Cache-Control", "no-cache")
		rw.Header().Set("Connection", "keep-alive")
		rw.Header().Set("Access-Control-Allow-Origin", "*")

		// Each connection registers its own message channel with the Broker's connections registry
		messageChan := make(chan []byte)

		// Signal the broker that we have a new connection
		broker.newClients <- messageChan

		// Remove this client from the map of connected clients
		// when this handler exits.
		defer func() {
			broker.closingClients <- messageChan
		}()

		// Listen to connection close and un-register messageChan
		notify := rw.(http.CloseNotifier).CloseNotify()

		go func() {
			<-notify
			broker.closingClients <- messageChan
		}()

		for {

			// Write to the ResponseWriter
			// Server Sent Events compatible
			fmt.Fprintf(rw, "data: %s\n\n", <-messageChan)

			// Flush the data immediatly instead of buffering it for later.
			flusher.Flush()
		}
	}

}

func (broker *Broker) listen() {
	for {
		select {
		case s := <-broker.newClients:

			// A new client has connected.
			// Register their message channel
			broker.clients[s] = true
			log.Printf("Client added. %d registered clients", len(broker.clients))
		case s := <-broker.closingClients:

			// A client has dettached and we want to
			// stop sending them messages.
			delete(broker.clients, s)
			log.Printf("Removed client. %d registered clients", len(broker.clients))
		case event := <-broker.Notifier:

			// We got a new event from the outside!
			// Send event to all connected clients
			for clientMessageChan, _ := range broker.clients {
				clientMessageChan <- event
			}
		}
	}

}

func main() {

	broker := NewServer()

	// go func() {
	// 	for {
	// 		time.Sleep(time.Second * 2)
	// 		eventString := fmt.Sprintf("the time is %v", time.Now())
	// 		log.Println("Receiving event")
	// 		broker.Notifier <- []byte(eventString)
	// 	}
	// }()
	// localhost:3000, 192.168.33.10
  //id=1&st=0&t=0000
	fmt.Println("Para enviar peticiones dispositivos: curl -v 192.168.0.18:9000/dev?data=50FD7888D005DC5231C3474F844600AB")
  //id=1&st=0&t=0000
  fmt.Println("Para enviar peticiones dispositivos: curl -v 192.168.0.18:9000/dev?data=1D211AA948D6208B2F33BB632709C3C6")
  //id=2&st=0&t=0000
  fmt.Println("Para enviar peticiones dispositivos: curl -v 192.168.0.18:9000/dev?data=2E54E91F0812A12ACDD3C095A050C8C1")
  //id=2&st=1&t=0000
  fmt.Println("Para enviar peticiones dispositivos: curl -v 192.168.0.18:9000/dev?data=BA2211AF71E8FA145BCAAFA8E496542D")
  fmt.Println("Para enviar peticiones cliente: curl -v 192.168.0.18:9000")
	log.Fatal("HTTP server error: ", http.ListenAndServe(":9000", broker))

}

