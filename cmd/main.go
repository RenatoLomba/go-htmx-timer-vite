package main

import (
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// It keeps a list of clients those are currently attached
// and broadcasting events to those clients.
type Event struct {
	// Events are pushed to this channel by the main events-gathering routine
	Message chan string

	// New client connections
	NewClients chan chan string

	// Closed client connections
	ClosedClients chan chan string

	// Total client connections
	TotalClients map[chan string]bool
}

// New event messages are broadcast to all registered client connection channels
type ClientChan chan string

type Timing struct {
	Start time.Time
	Stop  time.Time
}

var timings = make([]Timing, 0)

func main() {
	router := gin.Default()

	// Parse Static files
	router.LoadHTMLGlob("./public/views/*.html")

	// Parse Static folders
	router.Static("/static", "./public/dist")
	router.Static("/libs", "./public/libs")

	router.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})
	router.GET("/healthcheck", func(c *gin.Context) {
		c.String(200, "Server is OK")
	})

	// Initialize new streaming server
	stream := NewServer()

	go func() {
		for {
			if len(timings) == 0 {
				continue
			}

			timing := timings[len(timings)-1]
			time.Sleep(time.Second * 1)
			message := getSecondsOrTimingFormatted(timing)
			stream.Message <- message
		}
	}()

	// Client can stream the event
	// Add event-streaming headers
	router.GET("/events", HeadersMiddleware(), stream.serveHTTP(), func(c *gin.Context) {
		v, ok := c.Get("clientChan")
		if !ok {
			return
		}
		clientChan, ok := v.(ClientChan)
		if !ok {
			return
		}
		c.Stream(func(w io.Writer) bool {
			// Stream message to client from message channel
			if msg, ok := <-clientChan; ok {
				c.SSEvent("message", msg)
				return true
			}
			return false
		})
	})
	router.POST("/new-timing", func(c *gin.Context) {
		timing := Timing{
			Start: time.Now(),
		}
		timings = append(timings, timing)

		stream.Message <- getSecondsAfterStartFormatted(timing)
		c.String(200, "<button hx-patch=\"/stop-timing\">Stop</button>")
	})
	router.PATCH("/stop-timing", func(c *gin.Context) {
		timings[len(timings)-1].Stop = time.Now()

		c.String(200, "<button hx-post=\"/new-timing\">Start</button>")
	})

	log.Fatal(router.Run(":8080"))
}

func getSecondsOrTimingFormatted(timing Timing) string {
	var message string

	if timing.Stop.IsZero() {
		message = getSecondsAfterStartFormatted(timing)
	} else {
		var sb strings.Builder

		for idx, timing := range timings {
			sb.WriteString(fmt.Sprintf("%d. %s - %s\n", idx+1, timing.Start.Format(time.RFC3339), timing.Stop.Format(time.RFC3339)))
		}

		message = sb.String()
	}

	return message
}

func getSecondsAfterStartFormatted(timing Timing) string {
	secondsAfterStart := time.Since(timing.Start).Seconds()
	return strconv.FormatFloat(secondsAfterStart, 'f', 0, 64)
}

// Initialize event and Start procnteessing requests
func NewServer() (event *Event) {
	event = &Event{
		Message:       make(chan string),
		NewClients:    make(chan chan string),
		ClosedClients: make(chan chan string),
		TotalClients:  make(map[chan string]bool),
	}

	go event.listen()

	return
}

// It Listens all incoming requests from clients.
// Handles addition and removal of clients and broadcast messages to clients.
func (stream *Event) listen() {
	for {
		select {
		// Add new available client
		case client := <-stream.NewClients:
			stream.TotalClients[client] = true
			log.Printf("Client added. %d registered clients", len(stream.TotalClients))

		// Remove closed client
		case client := <-stream.ClosedClients:
			delete(stream.TotalClients, client)
			close(client)
			log.Printf("Removed client. %d registered clients", len(stream.TotalClients))

		// Broadcast message to client
		case eventMsg := <-stream.Message:
			for clientMessageChan := range stream.TotalClients {
				clientMessageChan <- eventMsg
			}
		}
	}
}

func (stream *Event) serveHTTP() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Initialize client channel
		clientChan := make(ClientChan)

		// Send new connection to event server
		stream.NewClients <- clientChan

		defer func() {
			// Send closed connection to event server
			stream.ClosedClients <- clientChan
		}()

		c.Set("clientChan", clientChan)

		c.Next()
	}
}

func HeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Transfer-Encoding", "chunked")
		c.Next()
	}
}
