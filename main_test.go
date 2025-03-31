package main

import (
	"github.com/google/uuid"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/tarantool/go-tarantool"
)

const (
	testMattermostURL   = "http://mattermost:8065"
	testMattermostToken = "your_bot_token"
	testTarantoolAddr   = "tarantool:3301"
	testTarantoolUser   = "mm_bot"
	testTarantoolPass   = "securepassword"
)

func setupTestBot(t *testing.T) *Bot {
	tarantoolConn, err := tarantool.Connect(testTarantoolAddr, tarantool.Opts{
		User: testTarantoolUser,
		Pass: testTarantoolPass,
	})
	if err != nil {
		t.Fatalf("Failed to connect to Tarantool: %v", err)
	}
	t.Cleanup(func() { tarantoolConn.Close() })

	bot, err := NewBot(testMattermostURL, testMattermostToken, tarantoolConn)
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	bot.client = model.NewAPIv4Client(testMattermostURL)
	bot.client.SetToken(testMattermostToken)

	return bot
}

func TestHandleVoteInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	bot := setupTestBot(t)

	voteID := "test_vote_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	_, err := bot.tarantool.Insert("votes", []interface{}{
		voteID,
		"test_channel",
		"test_user",
		"Test question?",
		map[string]int{"Option1": 0, "Option2": 0},
		time.Now().Unix(),
		true,
		map[string]bool{},
	})
	if err != nil {
		t.Fatalf("Failed to create test vote: %v", err)
	}

	t.Run("successful vote info", func(t *testing.T) {
		post := &model.Post{
			Message:   "/vote info " + voteID,
			ChannelId: "test_channel",
		}

		bot.handleVoteInfo(post)
	})

	t.Run("vote not found", func(t *testing.T) {
		post := &model.Post{
			Message:   "/vote info non_existent_vote",
			ChannelId: "test_channel",
		}

		bot.handleVoteInfo(post)
	})
}

func TestHandleCreateVote(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	bot := setupTestBot(t)

	t.Run("successful vote creation", func(t *testing.T) {
		post := &model.Post{
			Message:   `/vote create "Test question?" "Option1" "Option2"`,
			ChannelId: "test_channel",
			UserId:    "test_user",
		}

		bot.handleCreateVote(post)
	})

	t.Run("invalid format", func(t *testing.T) {
		post := &model.Post{
			Message:   `/vote create "Invalid format`,
			ChannelId: "test_channel",
		}

		bot.handleCreateVote(post)
	})
}

func TestHandleVote(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	bot := setupTestBot(t)

	voteID := "test_vote_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	_, err := bot.tarantool.Insert("votes", []interface{}{
		voteID,
		"test_channel",
		"test_user",
		"Test question?",
		map[string]int{"Option1": 0, "Option2": 0},
		time.Now().Unix(),
		true,
		map[string]bool{},
	})

	if err != nil {
		t.Fatalf("Failed to create test vote: %v", err)
	}

	t.Run("successful vote", func(t *testing.T) {
		post := &model.Post{
			Message:   `/vote "Option1" ` + voteID,
			ChannelId: "test_channel",
			UserId:    "voting_user",
		}

		bot.handleVote(post)
	})

	t.Run("vote not found", func(t *testing.T) {
		post := &model.Post{
			Message:   `/vote "Option1" non_existent_vote`,
			ChannelId: "test_channel",
		}

		bot.handleVote(post)
	})
}

func TestHandleEndVote(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	bot := setupTestBot(t)

	voteID := "test_vote_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	_, err := bot.tarantool.Insert("votes", []interface{}{
		voteID,
		"test_channel",
		"test_user",
		"Test question?",
		map[string]int{"Option1": 3, "Option2": 2},
		time.Now().Unix(),
		true,
		map[string]bool{},
	})
	if err != nil {
		t.Fatalf("Failed to create test vote: %v", err)
	}

	t.Run("successful end vote", func(t *testing.T) {
		post := &model.Post{
			Message:   "/vote end " + voteID,
			ChannelId: "test_channel",
			UserId:    "test_user",
		}

		bot.handleEndVote(post)
	})

	t.Run("not creator", func(t *testing.T) {
		post := &model.Post{
			Message:   "/vote end " + voteID,
			ChannelId: "test_channel",
			UserId:    "other_user",
		}

		bot.handleEndVote(post)
	})
}

func TestHandleDeleteVote(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	bot := setupTestBot(t)

	voteID := "test_vote_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	_, err := bot.tarantool.Insert("votes", []interface{}{
		voteID,
		"test_channel",
		"test_user",
		"Test question?",
		map[string]int{"Option1": 3, "Option2": 2},
		time.Now().Unix(),
		true,
		map[string]bool{},
	})

	if err != nil {
		t.Fatalf("Failed to create test vote: %v", err)
	}

	t.Run("successful delete", func(t *testing.T) {
		post := &model.Post{
			Message:   "/vote delete " + voteID,
			ChannelId: "test_channel",
			UserId:    "test_user",
		}

		bot.handleDeleteVote(post)
	})

	t.Run("delete error", func(t *testing.T) {
		post := &model.Post{
			Message:   "/vote delete non_existent_vote",
			ChannelId: "test_channel",
			UserId:    "test_user",
		}

		bot.handleDeleteVote(post)
	})
}

func TestMain(m *testing.M) {
	log.Println("Setting up test environment...")

	code := m.Run()

	log.Println("Tests completed")
	os.Exit(code)
}
