package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/tarantool/go-tarantool"
	"log"
	"regexp"
	"strings"
	"time"
)

type Bot struct {
	client    *model.Client4
	wsClient  *model.WebSocketClient
	tarantool *tarantool.Connection
	user      *model.User
	team      *model.Team
}

func NewBot(serverURL, token string, tarantoolConn *tarantool.Connection) (*Bot, error) {
	client := model.NewAPIv4Client(serverURL)
	log.Printf("Created client for bot on %s", serverURL)

	client.SetToken(token)
	log.Printf("Set bot's token successfully")

	user, _, err := client.GetMe("")
	if err != nil {
		log.Printf("Error starting bot: %s", err)
		return nil, err
	}

	wsClient, err := model.NewWebSocketClient4(strings.Replace(serverURL, "http", "ws", 1), token)
	if err != nil {
		log.Printf("Error starting bot: %s", err)
		return nil, err
	}

	team, _, err := client.GetTeamByName("felco", "")
	if err != nil {
		log.Printf("Error starting bot: %s", err)
		return nil, err
	}

	return &Bot{
		client:    client,
		wsClient:  wsClient,
		tarantool: tarantoolConn,
		user:      user,
		team:      team,
	}, nil
}

func (b *Bot) Start() error {
	b.wsClient.Listen()

	log.Printf("Bot started successfully. Parsing user inputs...")

	for event := range b.wsClient.EventChannel {
		if event.EventType() != model.WebsocketEventPosted {
			continue
		}

		postData, ok := event.GetData()["post"].(string)
		if !ok {
			log.Println("Invalid post data format, waiting for the next message")
			continue
		}

		var post model.Post
		if err := json.Unmarshal([]byte(postData), &post); err != nil {
			log.Printf("Failed to parse post: %v, waiting for the next message...", err)
			continue
		}

		if post.UserId == b.user.Id {
			continue
		}

		b.handleMessage(&post)
	}
	return nil
}

func (b *Bot) handleMessage(post *model.Post) {
	log.Printf(post.Message)

	if post.UserId == b.user.Id {
		return
	}

	switch {
	case strings.HasPrefix(post.Message, "/vote create"):
		b.handleCreateVote(post)
	case strings.HasPrefix(post.Message, "/vote end"):
		b.handleEndVote(post)
	case strings.HasPrefix(post.Message, "/vote delete"):
		b.handleDeleteVote(post)
	case strings.HasPrefix(post.Message, "/vote info"):
		b.handleVoteInfo(post)
	case strings.HasPrefix(post.Message, "/vote"):
		b.handleVote(post)
	}
}

func (b *Bot) handleVoteInfo(post *model.Post) {
	log.Printf("Getting vote info for %s", post.UserId)

	re := regexp.MustCompile(`/vote info (\w+)`)
	matches := re.FindStringSubmatch(post.Message)
	if len(matches) < 2 {
		log.Printf("Not enough arguments, quitting...")
		b.sendHelp(post.ChannelId)
		return
	}

	voteID := matches[1]
	log.Printf("Got vote ID: " + voteID)

	log.Printf("Selecting from tarantool...")
	resp, err := b.tarantool.Select("votes", "primary", 0, 1, tarantool.IterEq, []interface{}{voteID})
	if err != nil || len(resp.Data) == 0 {
		log.Printf("Error getting vote: %v, quitting...", err)
		b.sendMessage(post.ChannelId, "Голосование не найдено")
		return
	}

	vote := resp.Data[0].([]interface{})
	log.Printf("Got vote info, building response...")

	options := vote[4].(map[interface{}]interface{})
	var resultText strings.Builder
	for opt, count := range options {
		resultText.WriteString(fmt.Sprintf("\n- %s: %d", opt, count))
	}

	question := vote[3].(string)
	message := fmt.Sprintf(
		"##### Текущие результаты голосования: %s\n%s",
		question,
		resultText.String(),
	)

	b.sendMessage(post.ChannelId, message)
	log.Printf("Sent response to client")
}

func (b *Bot) handleCreateVote(post *model.Post) {
	log.Printf("Creating vote for %s", post.UserId)

	re := regexp.MustCompile(`/vote create "([^"]+)" (".+")`)
	matches := re.FindStringSubmatch(post.Message)
	if len(matches) < 3 {
		log.Printf("Not enough arguments, quitting...")
		b.sendHelp(post.ChannelId)
		return
	}

	question := matches[1]
	optionsStr := matches[2]
	log.Printf("Got question and options: %s, %s", question, optionsStr)

	options := make(map[string]interface{})
	optionRe := regexp.MustCompile(`"([^"]+)"`)
	for _, match := range optionRe.FindAllStringSubmatch(optionsStr, -1) {
		if len(match) > 1 {
			options[match[1]] = 0
		}
	}
	log.Printf("Initialized options map")

	if len(options) < 2 {
		log.Printf("Not enough arguments, quitting...")
		b.sendMessage(post.ChannelId, "Нужно минимум 2 варианта ответа")
		return
	}

	voteID := strings.ReplaceAll(uuid.New().String(), "-", "")
	log.Printf("Generated unique vote ID: %s", voteID)

	voteData := []interface{}{
		voteID,
		post.ChannelId,
		post.UserId,
		question,
		options,
		time.Now().Unix(),
		true,
		make(map[string]bool),
	}
	log.Printf("Packed vote data")

	log.Printf("Selecting from tarantool...")
	_, err := b.tarantool.Insert("votes", voteData)
	if err != nil {
		log.Printf("Error creating vote: %v, quitting...", err)
		b.sendMessage(post.ChannelId, "Ошибка при создании голосования: "+err.Error())
		return
	}
	log.Printf("Inserted vote data successfully. Building response...")

	var optionsText strings.Builder
	for option := range options {
		optionsText.WriteString("\n- `" + option + "`")
	}

	message := fmt.Sprintf(
		"### %s\n**Варианты:**%s\n\nПроголосовать: `/vote \"ваш выбор\" %s`\n\nID голосования: `%s`",
		question,
		optionsText.String(),
		voteID,
		voteID,
	)

	b.sendMessage(post.ChannelId, message)
	log.Printf("Sent response to client")
}

func (b *Bot) handleVote(post *model.Post) {
	log.Printf("Processing voting for %s", post.UserId)

	re := regexp.MustCompile(`/vote "([^"]+)" (\w+)`)
	matches := re.FindStringSubmatch(post.Message)
	if len(matches) < 3 {
		log.Printf("Not enough arguments, quitting...")
		b.sendHelp(post.ChannelId)
		return
	}

	option := matches[1]
	voteID := matches[2]
	log.Printf("Got option voted for and vote ID: %s, %s", option, voteID)

	log.Printf("Selecting from tarantool...")
	resp, err := b.tarantool.Select("votes", "primary", 0, 1, tarantool.IterEq, []interface{}{voteID})
	if err != nil || len(resp.Data) == 0 {
		log.Printf("Error finding vote: %v, quitting...", err)
		b.sendMessage(post.ChannelId, "Голосование не найдено")
		return
	}

	vote := resp.Data[0].([]interface{})
	log.Printf("Got vote info, checking the vote...")

	isActive, ok := vote[6].(bool)
	if !ok || !isActive {
		b.sendMessage(post.ChannelId, "Голосование уже завершено")
		return
	}
	log.Printf("Vote exists and not ended, checking for only once...")

	votedUsers, _ := vote[7].(map[interface{}]interface{})
	if votedUsers == nil {
		votedUsers = make(map[interface{}]interface{})
	}

	if _, voted := votedUsers[post.UserId]; voted {
		b.sendMessage(post.ChannelId, "Вы уже проголосовали в этом голосовании")
		log.Printf("User voted before, quitting...")
		return
	}
	log.Printf("User didn't vote, processing vote...")

	options, ok := vote[4].(map[interface{}]interface{})
	if !ok {
		log.Printf("Invalid options format, quitting...")
		b.sendMessage(post.ChannelId, "Ошибка формата голосования")
		return
	}

	if _, ok := options[option]; !ok {
		log.Printf("Invalid option, quitting...")
		b.sendMessage(post.ChannelId, "Недопустимый вариант")
		return
	}

	currentCount, _ := options[option].(int64)
	options[option] = currentCount + 1
	votedUsers[post.UserId] = true

	updateOps := []interface{}{
		[]interface{}{"=", 4, options},
		[]interface{}{"=", 7, votedUsers},
	}

	log.Printf("Updating space in tarantool...")
	_, err = b.tarantool.Update("votes", "primary", []interface{}{voteID}, updateOps)
	if err != nil {
		log.Printf("Error updating vote: %v, quitting...", err)
		b.sendMessage(post.ChannelId, "Ошибка при голосовании: "+err.Error())
		return
	}
	log.Printf("Vote info updated, user marked as voted. Building response...")

	var resultText strings.Builder
	for opt, count := range options {
		resultText.WriteString(fmt.Sprintf("\n- %s: %d", opt, count))
	}

	b.sendMessage(post.ChannelId, "Ваш голос учтён! Текущие результаты:"+resultText.String())
	log.Printf("Sent response to client")
}

func (b *Bot) handleDeleteVote(post *model.Post) {
	log.Printf("Deleting vote for %s", post.UserId)

	re := regexp.MustCompile(`/vote delete (\w+)`)
	matches := re.FindStringSubmatch(post.Message)
	if len(matches) < 2 {
		log.Printf("Not enough arguments, quitting...")
		b.sendHelp(post.ChannelId)
		return
	}

	voteID := matches[1]
	log.Printf("Got vote ID: %s", voteID)

	log.Printf("Selecting from tarantool...")
	resp, err := b.tarantool.Select("votes", "primary", 0, 1, tarantool.IterEq, []interface{}{voteID})
	if err != nil || len(resp.Data) == 0 {
		log.Printf("Error getting vote: %v, quitting...", err)
		b.sendMessage(post.ChannelId, "Голосование не найдено")
		return
	}
	log.Printf("Selected succesfully, preparing for deleting vote...")

	vote := resp.Data[0].([]interface{})
	creator := vote[2].(string)
	log.Printf("Got vote info. Checking for right to delete...")

	if creator != post.UserId {
		log.Printf("User has no right to delete other's vote, quitting...")
		b.sendMessage(post.ChannelId, "Только создатель может завершить голосование")
		return
	}
	log.Printf("Deleting vote...")

	log.Printf("Deleting from tarantool...")
	_, err = b.tarantool.Call("delete_vote", []interface{}{voteID, post.UserId})
	if err != nil {
		log.Printf("Error deleting vote: %v, quitting...", err)
		b.sendMessage(post.ChannelId, "Ошибка при удалении: "+err.Error())
		return
	}
	log.Printf("Deleted vote successfully. Building response...")

	b.sendMessage(post.ChannelId, "Голосование успешно удалено")
	log.Printf("Sent response to client")
}

func (b *Bot) handleEndVote(post *model.Post) {
	log.Printf("Handling end vote for %s", post.UserId)

	re := regexp.MustCompile(`/vote end (\w+)`)
	matches := re.FindStringSubmatch(post.Message)
	if len(matches) < 2 {
		log.Printf("Not enough arguments, quitting...")
		b.sendHelp(post.ChannelId)
		return
	}

	voteID := matches[1]
	log.Printf("Got vote ID: %s", voteID)

	log.Printf("Selecting from tarantool...")
	resp, err := b.tarantool.Select("votes", "primary", 0, 1, tarantool.IterEq, []interface{}{voteID})
	if err != nil || len(resp.Data) == 0 {
		log.Printf("Error getting vote: %v, quitting...", err)
		b.sendMessage(post.ChannelId, "Голосование не найдено")
		return
	}
	log.Printf("Selected succesfully, preparing for ending vote...")

	vote := resp.Data[0].([]interface{})
	creator := vote[2].(string)
	log.Printf("Got vote info. Checking for right to end...")

	if creator != post.UserId {
		log.Printf("User has no right to end other's vote, quitting...")
		b.sendMessage(post.ChannelId, "Только создатель может завершить голосование")
		return
	}
	log.Printf("Ending vote...")

	log.Printf("Calling tarantool to end the vote...")
	_, err = b.tarantool.Call("end_vote", []interface{}{voteID})
	if err != nil {
		log.Printf("Error ending vote: %v, quitting...", err)
		b.sendMessage(post.ChannelId, "Ошибка при завершении голосования: "+err.Error())
		return
	}
	log.Printf("Vote ended successfully. Building response...")

	options := vote[4].(map[interface{}]interface{})
	var resultText strings.Builder
	for opt, count := range options {
		resultText.WriteString(fmt.Sprintf("\n- %s: %d", opt, count))
	}

	question := vote[3].(string)
	message := fmt.Sprintf(
		"### Итоги голосования: %s\n**Результаты:**%s",
		question,
		resultText.String(),
	)

	b.sendMessage(post.ChannelId, message)
	log.Printf("Sent response to client")
}

func (b *Bot) sendHelp(channelID string) {
	log.Printf("Sending help message...")
	helpText := `**Доступные команды:**
- /vote create "Вопрос?" "Вариант1" "Вариант2" ... - Создать голосование
- /vote "Ваш выбор" ID - Проголосовать (1 раз)
- /vote info ID - Показать текущие результаты
- /vote end ID - Завершить голосование (только создатель)
- /vote delete ID - Удалить голосование (только создатель)`

	b.sendMessage(channelID, helpText)
	log.Printf("Sent help message to channel %s", channelID)
}

func (b *Bot) sendMessage(channelID, message string) {
	log.Printf("Sending message to channel %s...", channelID)

	post := &model.Post{
		ChannelId: channelID,
		Message:   message,
	}

	log.Printf("Creating post...")
	_, _, err := b.client.CreatePost(post)
	if err != nil {
		log.Printf("Error sending message: %v", err)
		return
	}
	log.Printf("Post created successfully")
}

func main() {
	log.Printf("Staring connection to tarantool...")
	tarantoolConn, err := tarantool.Connect("localhost:3301", tarantool.Opts{
		User: "mm_bot",
		Pass: "securepassword",
	})

	if err != nil {
		log.Fatalf("Failed to connect to Tarantool: %v", err)
	}
	log.Printf("Connected to tarantool! Starting bot...")

	defer tarantoolConn.Close()

	bot, err := NewBot(
		"http://localhost:8065",
		"1g47748hbfrz3b8qeapbfadgfe",
		tarantoolConn,
	)

	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	if err := bot.Start(); err != nil {
		log.Fatalf("Bot error: %v", err)
	}
}
