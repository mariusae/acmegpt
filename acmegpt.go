package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"9fans.net/go/acme"
	openai "github.com/sashabaranov/go-openai"
)

var win *acme.Win
var client *openai.Client
var ctx context.Context
var needchat = make(chan bool, 1)

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
	client = openai.NewClient(key)
	ctx = context.Background()

	var err error
	win, err = acme.New()
	if err != nil {
		log.Fatal(err)
	}
	// TODO: find existing windows, make each unique
	win.Name("+chatgpt")
	win.Ctl("clean")
	win.Fprintf("tag", "Get  ")

	go chat()

	for e := range win.EventChan() {
		if (e.C2 == 'x' || e.C2 == 'X') && string(e.Text) == "Get" {
			select {
			case needchat <- true:
			default:
			}
			continue
		}
		win.WriteEvent(e)
	}
}

func chat() {
	for range needchat {
		req := openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
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

		for {
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