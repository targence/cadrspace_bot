package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

const (
	statusURL    = "http://tun.targence.com:2000"
	calAccount   = "k63oqqu12qrmbo2giom17nu3m4@group.calendar.google.com"
	calURL       = "https://content.googleapis.com/calendar/v3/calendars/%s/events?timeMin=%s&timeMax=%s&key=%s"
	camStreamURL = "http://cadr.targence.com:4444/?action=snapshot.jpeg"
	camStaticURL = "http://cadr.targence.com:4444/?action=snapshot.jpeg"
)

var calKey = os.Getenv("CAL_KEY")
var token = os.Getenv("TG_TOKEN")
var zone, _ = time.LoadLocation("Europe/Moscow")

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

type db struct {
	ChatID    int64
	MessageID int
}

type status struct {
	State struct {
		Open bool `json:"open"`
	} `json:"state"`
}

type calendar struct {
	Items []struct {
		Summary string `json:"summary"`
		Start   struct {
			DateTime time.Time `json:"dateTime"`
			TimeZone string    `json:"timeZone"`
		} `json:"start"`
		End struct {
			DateTime time.Time `json:"dateTime"`
			TimeZone string    `json:"timeZone"`
		} `json:"end"`
	} `json:"items"`
}

func getBot() (*tgbotapi.BotAPI, tgbotapi.UpdatesChannel) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		logrus.Panic(err)
	}

	logrus.Printf("Telegram Authorized %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		logrus.Panic(err)
	}

	return bot, updates
}

func save(data *db) {
	file, err := os.Create("db.gob")
	if err != nil {
		logrus.Fatal(err)
	}

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		logrus.Fatal(err)
	}
	defer file.Close()
}

func load() db {
	data := db{}

	file, err := os.Open("db.gob")
	if err != nil {
		logrus.Warn(err)
	}

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		logrus.Warn(err)
	}
	defer file.Close()

	return data
}

var client = &http.Client{
	Timeout: time.Second * 5,
}

func getJSON(url string) []byte {

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.Panic(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		logrus.Panic(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Panic(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logrus.Fatal(err, resp.Status)
	}

	return body
}

func parseStatus(data []byte) status {
	var s = status{}
	err := json.Unmarshal(data, &s)
	if err != nil {
		logrus.Panic(err)
	}
	return s
}

func parseCal(data []byte) calendar {
	var c = calendar{}
	err := json.Unmarshal(data, &c)
	if err != nil {
		logrus.Panic(err)
	}
	return c
}

func genCalRequest() string {

	start := time.Now().In(zone).Format(time.RFC3339)
	end := time.Now().Add(7 * 24 * time.Hour).In(zone).Format(time.RFC3339)

	// url encode for "+"
	end = strings.Replace(end, "+", "%2B", -1)
	start = strings.Replace(start, "+", "%2B", -1)

	return fmt.Sprintf(calURL, calAccount, start, end, calKey)
}

func createMsg(bot *tgbotapi.BotAPI, storage *db) {

	msg := tgbotapi.NewMessage(storage.ChatID, fmt.Sprintf("Message test for chat ID: %d from bot", storage.ChatID))
	ok, err := bot.Send(msg)
	if err != nil {
		logrus.Fatal("Failed to create message", err)
	}

	storage.MessageID = ok.MessageID
	save(storage)
	logrus.Info("messageID: ", storage.ChatID, ", chatID: ", storage.MessageID, ", is saved in db.gob")
}

func changeMsg(status status, calendar calendar, bot *tgbotapi.BotAPI, storage db) {

	prefix := "🤖 Сейчас закрыто\n\n"
	if status.State.Open == true {
		prefix = "🤘 Сейчас открыто\n" + "📷 goo.gl/W9kUFN\n\n"
	}

	var msg string
	if len(calendar.Items) != 0 {
		msg = "⏰ График работы:\n"
		items := calendar.Items

		sort.Slice(items, func(i, j int) bool { return items[i].Start.DateTime.Before(items[j].Start.DateTime) })

		for _, item := range items {
			startTime := item.Start.DateTime.In(zone).Format("2 Jan, Mon 15:04")
			endTime := item.End.DateTime.In(zone).Format("15:04")
			msg = fmt.Sprintf("%s• %s~%s\n", msg, startTime, endTime)
		}

		logrus.Warn(msg)
	} else {
		msg = "⏰ График работы лучше уточнить у @avp"
	}

	edit := tgbotapi.NewEditMessageText(storage.ChatID, storage.MessageID, prefix+msg)
	_, err := bot.Send(edit)
	if err != nil && err.Error() != "Bad Request: message is not modified" {
		logrus.Info("Failed to edit message ", err)
	}

}

func main() {
	bot, updates := getBot()
	storage := load()

	if storage.ChatID == 0 || storage.MessageID == 0 {
		logrus.Info("Waiting for /register message...")
		for update := range updates {
			msg := update.Message
			reg := regexp.MustCompile(`^/register$`)
			if msg != nil && reg.MatchString(msg.Text) {
				logrus.Printf("[<-] Message %d in chat %d from (%s) %s", msg.MessageID, msg.Chat.ID, msg.From.UserName, msg.Text)
				storage.ChatID = msg.Chat.ID
				createMsg(bot, &storage)
				break
			} else {
				logrus.Info("Received command is not /register")
			}
		}
	}

	// Webcam inline query
	go func() {
		for update := range updates {
			inline := update.InlineQuery
			if inline != nil {

				var cams []interface{}
				msg := tgbotapi.NewInlineQueryResultPhotoWithThumb(inline.ID+"_1", camStaticURL, camStaticURL)
				cams = append(cams, msg)

				inlineConfig := tgbotapi.InlineConfig{
					InlineQueryID: inline.ID,
					IsPersonal:    false,
					CacheTime:     0,
					Results:       cams,
				}
				_, err := bot.Send(inlineConfig)
				if err != nil {
					logrus.Warn(err)
				}
			}

		}
	}()

	// Update message periodically
	for _ = range time.NewTicker(30 * time.Second).C {
		s := getJSON(statusURL)
		status := parseStatus(s)

		cURL := genCalRequest()
		c := getJSON(cURL)
		calendar := parseCal(c)

		changeMsg(status, calendar, bot, storage)
	}

}
