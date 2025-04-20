package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Game struct {
    OrganizerID int
    Players     map[int]string
    mu          sync.Mutex
}

var games = make(map[int64]*Game)

func main() {
    // Запускаем фиктивный HTTP-сервер в отдельной горутине
    go func() {
        port := os.Getenv("PORT")
        if port == "" {
            port = "8080" // Порт по умолчанию
        }
        log.Printf("Starting dummy HTTP server on port %s", port)
        http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
            w.Write([]byte("OK"))
        })
        log.Fatal(http.ListenAndServe(":"+port, nil))
    }()

    // Инициализация Telegram-бота
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

        switch text {
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

func startGame(bot *tgbotapi.BotAPI, chatID int64, userID int) {
    // Проверяем, не запущена ли уже игра
    if _, exists := games[chatID]; exists {
        sendMessage(bot, chatID, "Игра уже запущена. Завершите текущую игру, чтобы начать новую.")
        return
    }

    // Запускаем новую игру
    games[chatID] = &Game{
        OrganizerID: userID,
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

    // Проверяем, голосовал ли пользователь ранее
    if _, alreadyVoted := game.Players[userID]; alreadyVoted {
        sendMessage(bot, chatID, "Вы уже проголосовали!")
        return
    }

    // Сохраняем ответ пользователя
    game.Players[userID] = response
    sendMessage(bot, chatID, fmt.Sprintf("%s ответил '%s'.", botUserName(bot, userID), response))

    // Проверяем, достаточно ли проголосовало пользователей
    if len(game.Players) >= 3 {
        reportResults(bot, chatID, game)
        delete(games, chatID) // Очищаем данные игры
    }
}

func reportResults(bot *tgbotapi.BotAPI, chatID int64, game *Game) {
    var yesPlayers []string
    var noPlayers []string
    var laterPlayers []string

    for userID, response := range game.Players {
        if response == "буду" {
            yesPlayers = append(yesPlayers, botUserName(bot, userID))
        } else if response == "позже" {
            laterPlayers = append(laterPlayers, botUserName(bot, userID))
        } else {
            noPlayers = append(noPlayers, botUserName(bot, userID))
        }
    }

    result := fmt.Sprintf("Результаты:\nДа: %s\nНет: %s\nПозже: %s",
        strings.Join(yesPlayers, ", "),
        strings.Join(noPlayers, ", "),
        strings.Join(laterPlayers, ", "))
    sendMessage(bot, chatID, result)
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
    msg := tgbotapi.NewMessage(chatID, text)
    bot.Send(msg)
}

func botUserName(bot *tgbotapi.BotAPI, userID int) string {
    user, err := bot.GetChat(tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: int64(userID)}})
    if err != nil {
        log.Printf("Error fetching username for user %d: %v", userID, err)
        return fmt.Sprintf("Player %d", userID)
    }
    return user.UserName
}

func sendMessageWithKeyboard(bot *tgbotapi.BotAPI, chatID int64, text string, keyboard tgbotapi.ReplyKeyboardMarkup) {
    msg := tgbotapi.NewMessage(chatID, text)
    msg.ReplyMarkup = keyboard
    bot.Send(msg)
}

func respNoRainbow(bot *tgbotapi.BotAPI, chatID int64, userID int) {
    user, err := bot.GetChat(tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: int64(userID)}})
    if err != nil {
        sendMessage(bot, chatID, "Произошла ошибка при получении имени пользователя.")
        return
    }
    msg := fmt.Sprintf("Ну ладно, %s, заходи в другой раз!", "@"+user.UserName)
    sendMessage(bot, chatID, msg)
}