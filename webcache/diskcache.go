package webcache

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"time"
)

type DiskCache struct {
	Root string
	DeleteChannel chan *DiskCacheEntry
	SaveChannel chan *DiskCacheEntry
	journal *Journal
}

type DiskCacheEntry struct {
	Key string
	Value Value
	ExpirationTime time.Time
	ContentType string
	DoneChannel chan error
}

func NewDiskCache(cacheRoot string, logFile string) *DiskCache {
	if _, err := os.Stat(cacheRoot); os.IsNotExist(err) {
		log.Println(fmt.Sprintf("Diskcache root %s does not exist. Will create the diskcache root.", cacheRoot))
		err := os.MkdirAll(cacheRoot, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
	}
	journal, _ := NewJournal(logFile)
	dc := &DiskCache{
		Root: cacheRoot,
		DeleteChannel: make (chan *DiskCacheEntry),
		SaveChannel: make (chan *DiskCacheEntry),
		journal: journal,
	}

	go dc.Run()
	return dc
}

func (dc *DiskCache) Run() {
	for {
		select {
		case entry := <- dc.DeleteChannel:
			dc.Delete(entry)
		case entry := <- dc.SaveChannel:
			dc.Save(entry)
		}
	}
}

func (dc *DiskCache) Delete(entry *DiskCacheEntry) {
	dc.journal.Delete <- entry.Key
	deletePath := path.Join(dc.Root, entry.Key)
	log.Println(fmt.Sprintf("Deleting entry from disk. Key: %s", entry.Key))
	// Only returns error if path does not exist. This operation should
	// be idempotent so multiple calls to remove on the same file
	// shouldn't matter
	_ = os.Remove(deletePath)
	close(entry.DoneChannel)
}

func (dc *DiskCache) Save(entry *DiskCacheEntry) {
	response := &Response{
		Body:entry.Value,
		ContentType:entry.ContentType,
		ExpirationTime:entry.ExpirationTime,
	}
	b, err := marshal(response)
	if err != nil {
		entry.DoneChannel <- err
		close(entry.DoneChannel)
	}
	dc.journal.Add <- entry.Key
	f, err := os.Create(path.Join(dc.Root, entry.Key))
	if err != nil {
	} else {
		writer := bufio.NewWriter(f)
		_, err = writer.Write(b)
		writer.Flush()
		f.Sync()
		log.Println(fmt.Sprintf("Saved entry to disk. Key: %s", entry.Key))
	}
	f.Close()
	dc.journal.AddAck <- entry.Key
	entry.DoneChannel <- err
	close(entry.DoneChannel)
}


func (dc *DiskCache) Read(readChannel chan *DiskCacheEntry) {
	files, err := ioutil.ReadDir(dc.Root)
	if err != nil {
		log.Fatal(err)
	}

	validEntries := parseLogs(dc.journal.file)

	for _, f := range files {
		filename := f.Name()
		valid, ok := validEntries[filename]
		if !ok || !valid {
			log.Printf("Invalid file %s. Removing from disk.", filename)
			err := os.Remove(path.Join(dc.Root, filename))
			if err != nil {
				log.Println(err)
			}
			continue
		}

		b, err := ioutil.ReadFile(path.Join(dc.Root, filename))
		if err != nil {
			log.Fatal(err)
		}
		buf := bytes.NewBuffer(b)
		decoder := gob.NewDecoder(buf)
		resp, err := unmarshal(decoder)
		if err != nil {
			log.Fatal(err)
		}
		entry := &DiskCacheEntry{
			Key: f.Name(),
			Value: resp.Body,
			ExpirationTime:resp.ExpirationTime,
			ContentType:resp.ContentType}
		log.Println(fmt.Sprintf("Retrieved entry from disk. Key: %s", f.Name()))
		readChannel <- entry
	}
	close(readChannel)
}


func marshal(object interface{}) ([]byte, error) {
	var b bytes.Buffer
	gob.Register(Response{})
	enc := gob.NewEncoder(&b)
	err := enc.Encode(object)
	if err != nil { return nil, err }
	return b.Bytes(), nil
}

func unmarshal(dec *gob.Decoder) (*Response, error) {
	var resp Response
	err := dec.Decode(&resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func parseLogs(filename string) map[string]bool {
	f, err := os.OpenFile(filename, os.O_RDONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	entries := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.Split(scanner.Text()," ")
		action := line[0]
		key := line[1]

		switch action {
		case ADD:
			entries[key] = false
		case ADDACK:
			entries[key] = true
		case DELETE:
			delete(entries, key)
		}
	}

	return entries
}

type InvertedIndex struct {
	Filename   string
	Requests   chan MappingRequest
	NewMapping chan Mapping
}

type MappingRequest struct {
	hashed string
	response chan Result
}

type Mapping struct {
	Original string
	Hashed   string
}

type Result struct {
	original string
	ok bool
}

func (m *InvertedIndex) Run(loaded chan struct{}) chan struct{} {
	invertedMap := m.loadMapping(m.Filename)
	close(loaded)
	file, _ := os.OpenFile(m.Filename, os.O_CREATE | os.O_APPEND | os.O_WRONLY, 0644)

	for {
		select {
		case newEntry := <- m.NewMapping:
			_, ok := invertedMap[newEntry.Hashed]
			if !ok {
				invertedMap[newEntry.Hashed] = newEntry.Original
				fmt.Fprintf(file,"%s %s\n", newEntry.Hashed, newEntry.Original)
			}
		case request :=<- m.Requests:
			original, ok := invertedMap[request.hashed]
			request.response <- Result{original:original, ok:ok}
		}
	}
}

func (m *InvertedIndex) loadMapping(filename string) map[string]string {
	file, err := os.OpenFile(filename, os.O_RDONLY | os.O_CREATE | os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	entries := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.Split(scanner.Text()," ")
		hashed := line[0]
		original := line[1]
		entries[hashed] = original
	}
	return entries
}

func (m *InvertedIndex) Get(key string) (string, bool) {
	response := make(chan Result)
	m.Requests <- MappingRequest{hashed: strings.TrimPrefix(key, "http://"), response:response}
	res := <- response
	return res.original, res.ok
}

