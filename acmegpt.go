package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"9fans.net/go/acme"
	openai "github.com/sashabaranov/go-openai"
	"gopkg.in/yaml.v3"
)

var win *acme.Win
var client *openai.Client
var ctx context.Context
var needchat = make(chan bool, 1)
var needstop = make(chan bool, 1)
var model = openai.GPT4o // openai.GPT3Dot5Turbo

type config struct {
	Key   string `yaml:"key"`
	Model string `yaml:"model"`
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: acmegpt\n")
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("acmegpt: ")
	flag.Usage = usage
	flag.Parse()

	key := os.Getenv("OPENAI_API_KEY")

	file := path.Join(os.Getenv("HOME"), ".acmegpt")
	data, err := os.ReadFile(file)
	if err == nil {
		var conf config
		if err := yaml.Unmarshal(data, &conf); err != nil {
			log.Fatalf("unmarshal %s: %v", file, err)
		}
		key = conf.Key
		if conf.Model != "" {
			model = conf.Model
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Fatal(err)
	}

	client = openai.NewClient(key)
	ctx = context.Background()

	win, err = acme.New()
	if err != nil {
		log.Fatal(err)
	}
	win.Name("+chatgpt")
	win.Ctl("clean")
	win.Fprintf("tag", "Get Stop ")

	go chat()

Events:
	for e := range win.EventChan() {
		if e.C2 == 'x' || e.C2 == 'X' {
			switch string(e.Text) {
			case "Get":
				select {
				case needchat <- true:
				default:
				}
				continue Events
			case "Stop":
				select {
				case needstop <- true:
				default:
				}
				continue Events
			}
		}
		win.WriteEvent(e)
	}
}

func chat() {
	for range needchat {
		select {
		case <-needstop:
		default:
		}

		req := openai.ChatCompletionRequest{
			Model: model,
			// Do we need any system messages?
			Messages: readMessages(),
		}
		stream, err := client.CreateChatCompletionStream(ctx, req)
		//		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			log.Printf("openai: %v", err)
			continue
		}

		//		win.Addr("data", "$")
		//		win.Ctl("dot=addr")

		//		text := markdownfmt(resp.Choices[0].Message.Content)
		//		text = strings.Replace(text, "\n", "\n\t", -1)

		win.Addr("$")
		win.Write("data", []byte("\n\n\t"))
		//		win.Write("data", []byte(text))

	Read:
		for {
			select {
			case <-needstop:
				// In this case, abort.
				win.Write("data", []byte("<Stopped by user>"))
				break Read
			default:
			}
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				win.Write("data", []byte("\n"))
				break
			}
			if err != nil {
				log.Printf("openai: %v", err)
				break
			}

			out := strings.Replace(response.Choices[0].Delta.Content, "\n", "\n\t", -1)
			win.Write("data", []byte(out))
		}
		stream.Close()

		win.Write("data", []byte("\n"))
		win.Ctl("clean")
	}
}

func readMessages() (messages []openai.ChatCompletionMessage) {
	data, _ := win.ReadAll("body")
	// Segment body by indentation.
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		role := openai.ChatMessageRoleUser
		if line[0] == '\t' {
			role = openai.ChatMessageRoleAssistant
			line = line[1:]
		}
		if len(messages) == 0 || messages[len(messages)-1].Role != role {
			messages = append(messages, openai.ChatCompletionMessage{
				Role: role,
			})
		}
		m := &messages[len(messages)-1]
		m.Content = join(m.Content, line, "\n")
	}
	return
}

func join(left, right, delim string) string {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}
	return left + "\n" + right
}

/*
func markdownfmt(text string) string {
	const extensions = blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_AUTOLINK |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_SPACE_HEADERS |
		blackfriday.EXTENSION_NO_EMPTY_LINE_BEFORE_BLOCK

	return string(blackfriday.Markdown([]byte(text), markdown.NewRenderer(nil), extensions))
}
*/
