package webcache

import (
	"fmt"
	"log"
	"os"
)

const ADD = "ADD"
const ADDACK = "ADDACK"
const DELETE = "DELETE"

type Journal struct {
	Add chan string
	AddAck chan string
	Delete chan string
	file string
}

func NewJournal(filename string) (*Journal, error) {
	journal := &Journal{
		Add: make(chan string),
		AddAck: make(chan string),
		Delete: make(chan string),
		file: filename,
	}
	done := journal.Run()
	<- done
	return journal, nil
}

func (j *Journal) Run() chan struct{} {
	done := make(chan struct{})
	file, err := os.OpenFile(j.file, os.O_CREATE | os.O_APPEND | os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		defer file.Close()
		close(done)
		for {
			select {
			case entry := <- j.Add:
				j.log(file, ADD, entry)
			case entry := <- j.AddAck:
				j.log(file, ADDACK, entry)
			case entry := <- j.Delete:
				j.log(file, DELETE, entry)
			}
		}
		//file.Close()
	}()
	return done
}

func (j *Journal) log(file *os.File, action string, key string) {
	_, err := fmt.Fprintf(file,"%s %s\n", action, key)
	if err != nil {
		log.Println(err)
	}
}

