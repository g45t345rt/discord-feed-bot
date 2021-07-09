package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/radovskyb/watcher"
	"gopkg.in/yaml.v3"
)

type DiscordData struct {
	Content string         `json:"content"`
	Embeds  []DiscordEmbed `json:"embeds"`
}

type DiscordField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type DiscordEmbed struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Color       int            `json:"color"`
	Fields      []DiscordField `json:"fields,omitempty"`
}

type Config struct {
	Polling  int    `yaml:"polling"`
	Webhook  string `yaml:"webhook"`
	BasePath string `yaml:"folder"`
	WebLink  string `yaml:"webLink"`
}

var (
	config Config
	events []watcher.Event
)

func pollEvents(w *watcher.Watcher) {
	for {
		select {
		case event := <-w.Event:
			events = append(events, event)
		case err := <-w.Error:
			log.Fatalln(err)
		case <-w.Closed:
			return
		}
	}
}

func displayWatchedFiles(w *watcher.Watcher) {
	for path, f := range w.WatchedFiles() {
		fmt.Printf("%s: %s\n", path, f.Name())
	}
	fmt.Println()
}

func postWebhook(discordData DiscordData) {
	data, err := json.Marshal(discordData)
	if err != nil {
		fmt.Printf("json.Marshal %v\n", err)
	}

	req, err := http.NewRequest(
		"POST",
		config.Webhook,
		bytes.NewBuffer(data),
	)

	if err != nil {
		fmt.Printf("http.NewRequest %v\n", err)
		return
	}

	req.Header.Add("Content-Type", "application/json")

	c := &http.Client{}
	res, err := c.Do(req)

	if err != nil {
		fmt.Printf("http.Do %v\n", err)
		return
	}

	defer res.Body.Close()
}

func getRelPath(path string) string {
	path, err := filepath.Rel(config.BasePath, path)
	if err != nil {
		fmt.Printf("filepath.Rel %v\n", err)
		return "..."
	}

	return path
}

func dispatchEvents(w *watcher.Watcher) {
	for {
		if len(events) > 0 {
			newFilesEmbed := DiscordEmbed{
				Title: "New files",
				Color: 2667354,
			}

			deletedFilesEmbed := DiscordEmbed{
				Title: "Deleted files",
				Color: 14701138,
			}

			changeFileSet := DiscordEmbed{
				Title: "Changes",
				Color: 8750469,
			}

			for _, event := range events {
				fmt.Println(event)
				if event.IsDir() {
					continue
				}

				switch event.Op {
				case watcher.Remove:
					deletedFilesEmbed.Fields = append(deletedFilesEmbed.Fields, DiscordField{
						Name:  event.Name(),
						Value: "`" + getRelPath(event.Path) + "`",
					})
				case watcher.Create:
					value := "`" + getRelPath(event.Path) + "`\n"

					if config.WebLink != "" {
						value += "[web link](" + config.WebLink + url.QueryEscape(event.Path) + ")"
					}

					newFilesEmbed.Fields = append(newFilesEmbed.Fields, DiscordField{
						Name:  event.Name(),
						Value: value,
					})
				case watcher.Move:
					changeFileSet.Fields = append(changeFileSet.Fields, DiscordField{
						Name:  event.Name(),
						Value: "Move from `" + getRelPath(event.OldPath) + "` to `" + getRelPath(event.Path) + "`",
					})
				case watcher.Rename:
					changeFileSet.Fields = append(changeFileSet.Fields, DiscordField{
						Name:  event.Name(),
						Value: "Rename to `" + filepath.Base(event.Path),
					})
				}
			}

			msg := DiscordData{}
			if len(newFilesEmbed.Fields) > 0 {
				msg.Embeds = append(msg.Embeds, newFilesEmbed)
			}

			if len(deletedFilesEmbed.Fields) > 0 {
				msg.Embeds = append(msg.Embeds, deletedFilesEmbed)
			}

			if len(changeFileSet.Fields) > 0 {
				msg.Embeds = append(msg.Embeds, changeFileSet)
			}

			postWebhook(msg)
			events = events[:0]
		}

		time.Sleep(time.Duration(config.Polling) * time.Millisecond)
	}
}

func setConfig(conf *Config) {
	configFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	if err = yaml.Unmarshal(configFile, &conf); err != nil {
		log.Fatal(err)
	}
}

func main() {
	setConfig(&config)

	w := watcher.New()

	go pollEvents(w)
	go dispatchEvents(w)

	if err := w.AddRecursive(config.BasePath); err != nil {
		log.Fatalln(err)
	}

	//displayWatchedFiles(w)

	if err := w.Start(time.Duration(config.Polling) * time.Millisecond); err != nil {
		log.Fatalln(err)
	}
}
