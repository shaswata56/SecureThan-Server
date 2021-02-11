package main

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

type client struct {
	fName string
	lName string
	status bool
	conn net.Conn
}

var counter = struct {
	sync.RWMutex
	searchTable map[string]client
}{searchTable: make(map[string]client)}

func getString(bytes []byte) string {
	return hex.EncodeToString(bytes[:])
}

func socketPipe(c1, c2 client) {
	counter.RLock()
	conn1 := c1.conn
	conn2 := c2.conn
	counter.RUnlock()

	conn1.Write([]byte("ON"))
	conn2.Write([]byte("ON"))

	chan1 := chanFromConn(conn1)
	chan2 := chanFromConn(conn2)

	for {
		if conn1 == nil || conn2 == nil {
			conn1.Write([]byte("w:disconnected"))
			conn2.Write([]byte("w:disconnected"))
			counter.Lock()
			delete(counter.searchTable, c1.fName)
			delete(counter.searchTable, c2.fName)
			counter.Unlock()
		}
		select {
		case b1 := <- chan1:
			if b1 == nil {
				return
			} else {
				conn2.Write(b1)
			}
		case b2 := <- chan2:
			if b2 == nil {
				return
			} else {
				conn1.Write(b2)
			}
		}
	}
}

func chanFromConn(conn net.Conn) chan []byte {
	c := make(chan []byte)

	go func() {
		b := make([]byte, 1048576)

		for  {
			n, err := conn.Read(b)
			if n > 0 {
				res := make([]byte, n)
				copy(res, b[:n])
				c <- res
			}
			if err != nil {
				c <- nil
				break
			}
		}
	}()

	return c
}

func handleConnection(c net.Conn) {

	peerName := make([]byte, 128)
	fName := make([]byte, 64)
	lName := make([]byte, 64)
	n, err := c.Read(peerName)

	if err != nil {
		fmt.Errorf(err.Error())
	}
	if n >= 128 {
		fmt.Println("N size :",n)
	}
	for i := 0; i < 64; i++ {
		fName[i] = peerName[i]
	}
	for i := 64; i < 128; i++ {
		lName[i - 64] = peerName[i]
	}

	counter.RLock()
	if counter.searchTable[getString(fName)].status == true {
		warn := []byte("w:This username is already in use, try using something else!")
		c.Write(warn)
		counter.RUnlock()
		return
	}
	counter.RUnlock()

	c.Write([]byte("OK"))

	newClient := client{
		fName:  getString(fName),
		lName:  getString(lName),
		status: true,
		conn:      c,
	}

	counter.Lock()
	counter.searchTable[getString(fName)] = newClient
	counter.Unlock()

	counter.RLock()
	if counter.searchTable[getString(lName)].status == true {
		counter.RUnlock()
		return
	}
	counter.RUnlock()

	for {
		counter.RLock()
		if counter.searchTable[getString(lName)].status == true {
			counter.RUnlock()
			break
		}
		counter.RUnlock()
		time.Sleep(1 * time.Millisecond)
	}

	counter.RLock()
	c1 := counter.searchTable[getString(fName)]
	c2 := counter.searchTable[getString(lName)]
	counter.RUnlock()

	go socketPipe(c1, c2)
}

func main() {
	PORT := "0.0.0.0:5656"
	listener, err := net.Listen("tcp", PORT)
	if err != nil {
		fmt.Errorf(err.Error())
		return
	}
	defer listener.Close()
	rand.Seed(time.Now().Unix())

	for {
		c, err := listener.Accept()
		if err != nil {
			fmt.Errorf(err.Error())
			return
		}
		go handleConnection(c)
	}
}