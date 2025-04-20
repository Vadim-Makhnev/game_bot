package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

type Game struct {
    OrganizerID int
    Players     map[int]string 
    mu          sync.Mutex
}

var games = make(map[int64]*Game) 

func main() {
	err := godotenv.Load("./app")
	if err != nil {
		panic(err)
	}

    bot, err := tgbotapi.NewBotAPI(os.Getenv("TOKEN")) 
    if err != nil {
        log.Panic(err)
    }

    bot.Debug = true
    log.Printf("Authorized on account %s", bot.Self.UserName)

    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60

    updates := bot.GetUpdatesChan(u)

	keyboard := tgbotapi.NewReplyKeyboard(
        tgbotapi.NewKeyboardButtonRow(
            tgbotapi.NewKeyboardButton("Кто играть в радугу?"),
			tgbotapi.NewKeyboardButton("Не в радугу"),
        ),
			tgbotapi.NewKeyboardButtonRow(
            tgbotapi.NewKeyboardButton("Да"),
            tgbotapi.NewKeyboardButton("Нет"),
			tgbotapi.NewKeyboardButton("Позже"),
		),
	)

    for update := range updates {
        if update.Message == nil {
            continue
        }

        msg := update.Message
        chatID := msg.Chat.ID
        userID := msg.From.ID
        text := strings.ToLower(msg.Text)
		log.Println(chatID)

        switch strings.ToLower(text) {
        case "кто играть в радугу?":
            startGame(bot, chatID, int(userID))
        case "да":
            respondToGame(bot, chatID, int(userID), "буду")
        case "нет":
            respondToGame(bot, chatID, int(userID), "не буду")
		case "позже":
			respondToGame(bot, chatID, int(userID), "позже")
		case "не в радугу":
			respNoRainbow(bot, chatID, int(userID))
        default:
            sendMessageWithKeyboard(bot, chatID, "Выбери команду из меню.", keyboard)
        }
    }
}

func startGame(bot *tgbotapi.BotAPI, chatID int64, organizerID int) {
    games[chatID] = &Game{
        OrganizerID: organizerID,
        Players:     make(map[int]string),
    }

    message := "Игра начинается! Будете играть? Выберите или напишите 'Да', 'Нет' или 'Позже'."
    sendMessage(bot, chatID, message)
}

func respondToGame(bot *tgbotapi.BotAPI, chatID int64, userID int, response string) {
    game, exists := games[chatID]
    if !exists {
        sendMessage(bot, chatID, "Игра еще не началась. Напишите 'кто играть в радугу?', чтобы начать.")
        return
    }

    game.mu.Lock()
    defer game.mu.Unlock()

    game.Players[userID] = response
    sendMessage(bot, chatID, fmt.Sprintf("%s ответил '%s'.", botUserName(bot, userID), response))

    if len(game.Players) >= 3 { 
        reportResults(bot, chatID, game)
        delete(games, chatID) 
    }
}

func reportResults(bot *tgbotapi.BotAPI, chatID int64, game *Game) {
    var yesPlayers []string
    var noPlayers []string
	var laterPlayers[]string

    for userID, response := range game.Players {
        if response == "буду" {
            yesPlayers = append(yesPlayers, botUserName(bot, userID))
        } else if response == "позже"{
			laterPlayers = append(laterPlayers, botUserName(bot, userID))
		}else {
            noPlayers = append(noPlayers, botUserName(bot, userID))
        }
    }

    result := fmt.Sprintf("Результаты:\nДа: %s\nНет: %s \nПозже: %s", strings.Join(yesPlayers, ", "), strings.Join(noPlayers, ", "), strings.Join(laterPlayers, ", "))
    sendMessage(bot, chatID, result)
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
    msg := tgbotapi.NewMessage(chatID, text)
    bot.Send(msg)
}

func botUserName(bot *tgbotapi.BotAPI, userID int) string {
    user, err := bot.GetChat(tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: int64(userID)}})
    if err != nil {
        return fmt.Sprintf("Player %d", userID)
    }
    return user.UserName
}

func sendMessageWithKeyboard(bot *tgbotapi.BotAPI, chatID int64, text string, keyboard tgbotapi.ReplyKeyboardMarkup) {
    msg := tgbotapi.NewMessage(chatID, text)
    msg.ReplyMarkup = keyboard
    bot.Send(msg)
}

func respNoRainbow(bot *tgbotapi.BotAPI, chatID int64, userID int ) {
	var msg string
	user, err := bot.GetChat(tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: int64(userID)}})
	if err != nil {
        msg = "error"
    }
	msg = fmt.Sprintf("Ну а нафиг ты тогда зашел сюда? %s", "@" + user.UserName)
	sendMessage(bot, chatID, msg)
}