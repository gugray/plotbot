package main

import (
	"encoding/json"
	"fmt"
	"github.com/disgoorg/disgo/webhook"
	"github.com/disgoorg/snowflake/v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const configVarName = "CONFIG"
const devConfigPath = "config.dev.json"
const streamReconnectSec = 15
const streamReadBufSz = 65536
const heartbeatSec = 30

var cfg Config

var readingStream = false

func readConfig() {

	cfgPath := os.Getenv(configVarName)
	if len(cfgPath) == 0 {
		cfgPath = devConfigPath
	}
	cfgJson, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(cfgJson, &cfg); err != nil {
		panic(err)
	}
}

func blockUntilSignal() {
	signalChannel := make(chan os.Signal, 2)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	for {
		sig := <-signalChannel
		switch sig {
		case os.Interrupt:
			fmt.Println("sigint")
		case syscall.SIGTERM:
			fmt.Println("sigterm")
		}
		break
	}
}

func heartbeat(url string, freqSec int) {

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Failed to make heartbeat request; no heartbeat.")
		return
	}
	for {
		time.Sleep(time.Duration(freqSec) * time.Second)
		// Not sending heartbeat if stream is not working - so uptime monitor alerts us
		if !readingStream {
			continue
		}
		client.Do(req)
	}
}

func readStreamWithRetry(address string, msgs chan<- string) {

	for {
		log.Printf("Connecting to stream: %v", address)
		readStream(address, msgs)
		readingStream = false
		log.Printf("Disconnected from stream. Waiting %v seconds before reconnecting.", streamReconnectSec)
		time.Sleep(streamReconnectSec * time.Second)
	}
}

func readStream(address string, msgs chan<- string) {

	req, err := http.NewRequest("GET", address, nil)
	if err != nil {
		log.Print(err)
		return
	}

	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Print(err)
		return
	}

	log.Println("Connected to stream. Now reading.")
	readingStream = true

	data := make([]byte, streamReadBufSz)
	var currlnBytes []byte
	var lines []string
	for {
		n, err := resp.Body.Read(data)
		if err != nil {
			log.Print(err)
			return
		}
		for i := 0; i < n; i++ {
			b := data[i]
			if b != '\n' {
				currlnBytes = append(currlnBytes, b)
			} else {
				lines = append(lines, string(currlnBytes))
				currlnBytes = currlnBytes[:0]
			}
		}
		for len(lines) > 0 {
			line := lines[0]
			lines = lines[1:]
			msgs <- line
		}
	}
}

func handleMsgs(msgs <-chan string, updates chan<- string) {

	var currEvent string
	for {
		msg := <-msgs
		if len(msg) == 0 || strings.HasPrefix(msg, ":") {
			currEvent = ""
			continue
		}
		if strings.HasPrefix(msg, "event: ") {
			currEvent = msg[7:]
			continue
		}
		if strings.HasPrefix(msg, "data: ") && currEvent == "update" {
			data := msg[6:]
			updates <- data
			continue
		}
		currEvent = ""
	}
}

func relayUpdates(updates <-chan string, hookId uint64, hookToken, defaultInstace string) {

	client := webhook.New(snowflake.ID(hookId), hookToken)

	for {
		updStr := <-updates
		//fmt.Println(updStr)

		var upd Update
		if err := json.Unmarshal([]byte(updStr), &upd); err != nil {
			log.Printf("Failed to parse update: %v", err)
			continue
		}
		if strings.IndexByte(upd.Account.Account, '@') == -1 {
			upd.Account.Account += defaultInstace
		}

		log.Printf("Toot by %v at %v", upd.Account.Account, upd.CreatedAt)

		content := fmt.Sprintf("%v (%v) tooted this: %v", upd.Account.DisplayName, upd.Account.Account, upd.URL)
		if _, err := client.CreateContent(content); err != nil {
			log.Printf("Failed to post to Discord: %v", err)
			continue
		}
	}
}

func main() {

	readConfig()

	msgs := make(chan string, 2)
	updates := make(chan string, 2)

	go heartbeat(cfg.HearbeatUrl, heartbeatSec)
	go readStreamWithRetry(cfg.StreamUrl, msgs)
	go handleMsgs(msgs, updates)
	go relayUpdates(updates, cfg.DiscordWebhookId, cfg.DiscordWebhookToken, cfg.DefaultAcctInstance)

	blockUntilSignal()
}
