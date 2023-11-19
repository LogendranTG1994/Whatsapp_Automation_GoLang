package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type MyClient struct {
	WAClient       *whatsmeow.Client
	eventHandlerID uint32
}

func (mycli *MyClient) register() {
	mycli.eventHandlerID = mycli.WAClient.AddEventHandler(mycli.eventHandler)
}

func (mycli *MyClient) eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		newMessage := v.Message
		msg := newMessage.GetConversation()
		fmt.Println("Message from:", v.Info.Sender.User, "->", msg)
		if msg == "" {
			return
		}

		// Make the message comparison case-insensitive
		msg = strings.ToLower(msg)

		// Define predefined responses
		responses := map[string]string{
			"hello":            "Hi there!",
			"how are you":      "I'm doing well, thank you!",
			"what's your name": "I am a bot.",
			// Add more responses as needed
		}

		// Check if the message is a predefined response
		if response, exists := responses[msg]; exists {
			sendResponse(mycli, v.Info.Sender.User, response)
			return
		}

		// Write the received message to the log file
		writeToLog(v.Info.Sender.User, msg)

		// Send the message content to another file
		sendMessageToFile(msg)
	}
}

func sendResponse(mycli *MyClient, userJid string, response string) {
	responseMessage := &waProto.Message{Conversation: proto.String(response)}
	userJidObj := types.NewJID(userJid, types.DefaultUserServer)
	mycli.WAClient.SendMessage(context.Background(), userJidObj, responseMessage)
}

func writeToLog(senderMobile, message string) {
	file, err := os.OpenFile("message_log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// Format the current date and time
	currentTime := time.Now().Format("2006-01-02 15:04:05")

	// Write the sender's mobile number, message, and date-time to the log file
	logEntry := fmt.Sprintf("[%s] Sender: %s\nMessage: %s\n\n", currentTime, senderMobile, message)
	if _, err := file.WriteString(logEntry); err != nil {
		fmt.Println("Error writing to file:", err)
	}
}

func sendMessageToFile(message string) {
	file, err := os.OpenFile("message_content.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// Write the message content to the file
	if _, err := file.WriteString(message + "\n"); err != nil {
		fmt.Println("Error writing to file:", err)
	}
}

func main() {
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	container, err := sqlstore.New("sqlite3", "file:examplestore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}

	clientLog := waLog.Stdout("Client", "DEBUG", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	mycli := &MyClient{WAClient: client}
	mycli.register()

	qrChan, _ := client.GetQRChannel(context.Background())
	err = client.Connect()
	if err != nil {
		panic(err)
	}

	for evt := range qrChan {
		if evt.Event == "code" {
			// Render the QR code here
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
		} else {
			fmt.Println("Login event:", evt.Event)
		}
	}

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}
