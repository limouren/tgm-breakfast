package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func mustGetEnv(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	panic(fmt.Sprintf("got empty os.Getenv(%#v)", key))
}

func mustGetLocations(key string) []string {
	rawLocations := mustGetEnv(key)
	return strings.Split(rawLocations, ",")
}

func indexFromMon(weekday time.Weekday) int {
	switch weekday {
	case time.Saturday:
		return 5
	case time.Sunday:
		return 6
	default:
		return int(weekday) - 1
	}
}

// Update is a telegram update object
type Update struct {
	ID      int      `json:"update_id"`
	Message *Message `json:"message"`
}

// Message is a telegram message object
type Message struct {
	ID   int           `json:"message_id"`
	Chat UserGroupChat `json:"chat"`
	Text string        `json:"text"`
}

// UserGroupChat is either a User or GroupChat object
// We use only the id field here
type UserGroupChat struct {
	ID int `json:"id"`
}

// Server serves breakfast!
type Server struct {
	Token     string
	Locations []string
	Loc       *time.Location
}

// HandleToday tells the breakfast location today
func (s *Server) HandleToday(w http.ResponseWriter, r *http.Request) {
	var update Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Panicf("Failed to unmarshal update: %v", err)
	}

	message := s.deriveMessage(update)

	form := url.Values{}
	form.Set("chat_id", strconv.Itoa(update.Message.Chat.ID))
	form.Set("text", message)
	form.Set("reply_to_message_id", strconv.Itoa(update.Message.ID))

	client := http.Client{}
	resp, err := client.PostForm(fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.Token), form)
	if err != nil {
		log.Panicf("Failed to sendMessage: %v", err)
	} else if resp.StatusCode < 200 && resp.StatusCode > 299 {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Panicf("got status code = %v\nBody = %v", resp.StatusCode, string(body))
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) deriveMessage(update Update) string {
	weekday := time.Now().In(s.Loc).Weekday()
	index := indexFromMon(weekday)

	isTomorrow := strings.Contains(strings.ToLower(update.Message.Text), "tomorrow")
	if isTomorrow {
		index++
	}

	if index < 6 {
		return s.Locations[index]
	} else {
		var day string
		if isTomorrow {
			day = "tomorrow"
		} else {
			day = "today"
		}

		return fmt.Sprintf("No breakfast for you %s :)", day)
	}
}

func Root(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello World :)")
}

func main() {
	port := mustGetEnv("PORT")
	token := mustGetEnv("TGMBK_TOKEN")
	locations := mustGetLocations("TGMBK_LOCATIONS")

	loc, err := time.LoadLocation("Asia/Hong_Kong")
	if err != nil {
		log.Printf("Failed to obtain location: %v", err)
	}

	server := Server{token, locations, loc}

	http.Handle("/", http.HandlerFunc(Root))
	http.Handle("/"+token, http.HandlerFunc(server.HandleToday))
	log.Println("Listening on", token)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
