package main

import (
	"errors"
	"fmt"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/tarantool/go-tarantool"
	"strings"
	"testing"
)

type MockTarantoolConnection struct {
	SelectFunc func(space, index interface{}, offset, limit uint32, iterator tarantool.Iter, key interface{}) (*tarantool.Response, error)
	InsertFunc func(space interface{}, tuple interface{}) (*tarantool.Response, error)
	UpdateFunc func(space, index interface{}, key, ops interface{}) (*tarantool.Response, error)
	CallFunc   func(functionName string, args interface{}) (*tarantool.Response, error)
}

func (m *MockTarantoolConnection) Select(space, index interface{}, offset, limit uint32, iterator tarantool.Iter, key interface{}) (*tarantool.Response, error) {
	return m.SelectFunc(space, index, offset, limit, iterator, key)
}

func (m *MockTarantoolConnection) Insert(space interface{}, tuple interface{}) (*tarantool.Response, error) {
	return m.InsertFunc(space, tuple)
}

func (m *MockTarantoolConnection) Update(space, index interface{}, key, ops interface{}) (*tarantool.Response, error) {
	return m.UpdateFunc(space, index, key, ops)
}

func (m *MockTarantoolConnection) Call(functionName string, args interface{}) (*tarantool.Response, error) {
	return m.CallFunc(functionName, args)
}

type MockMattermostClient struct {
	CreatePostFunc func(post *model.Post) (*model.Post, *model.Response, error)
}

func (m *MockMattermostClient) CreatePost(post *model.Post) (*model.Post, *model.Response, error) {
	return m.CreatePostFunc(post)
}

func TestHandleVoteInfo(t *testing.T) {
	tests := []struct {
		name          string
		message       string
		tarantoolMock func() *MockTarantoolConnection
		expectedError string
	}{
		{
			name:    "successful vote info",
			message: "/vote info abc123",
			tarantoolMock: func() *MockTarantoolConnection {
				return &MockTarantoolConnection{
					SelectFunc: func(space, index interface{}, offset, limit uint32, iterator tarantool.Iter, key interface{}) (*tarantool.Response, error) {
						return &tarantool.Response{
							Data: [][]interface{}{
								{
									"abc123",
									"channel1",
									"user1",
									"Test question?",
									map[interface{}]interface{}{"Option1": 5, "Option2": 3},
									int64(1234567890),
									true,
									map[string]bool{},
								},
							},
						}, nil
					},
				}
			},
		},
		{
			name:    "vote not found",
			message: "/vote info notfound",
			tarantoolMock: func() *MockTarantoolConnection {
				return &MockTarantoolConnection{
					SelectFunc: func(space, index interface{}, offset, limit uint32, iterator tarantool.Iter, key interface{}) (*tarantool.Response, error) {
						return &tarantool.Response{Data: [][]interface{}{}}, nil
					},
				}
			},
			expectedError: "Голосование не найдено",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			post := &model.Post{
				Message:   tt.message,
				ChannelId: "test-channel",
			}

			bot := &Bot{
				tarantool: tt.tarantoolMock(),
				client: &MockMattermostClient{
					CreatePostFunc: func(post *model.Post) (*model.Post, *model.Response, error) {
						if tt.expectedError != "" && post.Message == tt.expectedError {
							return nil, nil, nil
						}
						return nil, nil, nil
					},
				},
				user: &model.User{Id: "bot-user"},
			}

			bot.handleVoteInfo(post)
		})
	}
}

func TestHandleCreateVote(t *testing.T) {
	tests := []struct {
		name          string
		message       string
		tarantoolMock func() *MockTarantoolConnection
		expectedError string
	}{
		{
			name:    "successful vote creation",
			message: `/vote create "Test question?" "Option1" "Option2"`,
			tarantoolMock: func() *MockTarantoolConnection {
				return &MockTarantoolConnection{
					InsertFunc: func(space interface{}, tuple interface{}) (*tarantool.Response, error) {
						return &tarantool.Response{}, nil
					},
				}
			},
		},
		{
			name:          "invalid format",
			message:       `/vote create invalid-format`,
			expectedError: "Нужно минимум 2 варианта ответа",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			post := &model.Post{
				Message:   tt.message,
				ChannelId: "test-channel",
				UserId:    "user1",
			}

			bot := &Bot{
				tarantool: tt.tarantoolMock(),
				client: &MockMattermostClient{
					CreatePostFunc: func(post *model.Post) (*model.Post, *model.Response, error) {
						if tt.expectedError != "" && post.Message == tt.expectedError {
							return nil, nil, nil
						}
						return nil, nil, nil
					},
				},
				user: &model.User{Id: "bot-user"},
			}

			bot.handleCreateVote(post)
		})
	}
}

func TestHandleVote(t *testing.T) {
	tests := []struct {
		name          string
		message       string
		tarantoolMock func() *MockTarantoolConnection
		expectedError string
	}{
		{
			name:    "successful vote",
			message: `/vote "Option1" abc123`,
			tarantoolMock: func() *MockTarantoolConnection {
				return &MockTarantoolConnection{
					SelectFunc: func(space, index interface{}, offset, limit uint32, iterator tarantool.Iter, key interface{}) (*tarantool.Response, error) {
						return &tarantool.Response{
							Data: [][]interface{}{
								{
									"abc123",
									"channel1",
									"user1",
									"Test question?",
									map[interface{}]interface{}{"Option1": 5, "Option2": 3},
									int64(1234567890),
									true,
									map[string]bool{},
								},
							},
						}, nil
					},
					UpdateFunc: func(space, index interface{}, key, ops interface{}) (*tarantool.Response, error) {
						return &tarantool.Response{}, nil
					},
				}
			},
		},
		{
			name:    "vote not found",
			message: `/vote "Option1" notfound`,
			tarantoolMock: func() *MockTarantoolConnection {
				return &MockTarantoolConnection{
					SelectFunc: func(space, index interface{}, offset, limit uint32, iterator tarantool.Iter, key interface{}) (*tarantool.Response, error) {
						return &tarantool.Response{Data: [][]interface{}{}}, nil
					},
				}
			},
			expectedError: "Голосование не найдено",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			post := &model.Post{
				Message:   tt.message,
				ChannelId: "test-channel",
				UserId:    "user1",
			}

			bot := &Bot{
				tarantool: tt.tarantoolMock(),
				client: &MockMattermostClient{
					CreatePostFunc: func(post *model.Post) (*model.Post, *model.Response, error) {
						if tt.expectedError != "" && post.Message == tt.expectedError {
							return nil, nil, nil
						}
						return nil, nil, nil
					},
				},
				user: &model.User{Id: "bot-user"},
			}

			bot.handleVote(post)
		})
	}
}

func TestHandleEndVote(t *testing.T) {
	tests := []struct {
		name          string
		message       string
		userId        string
		tarantoolMock func() *MockTarantoolConnection
		expectedError string
	}{
		{
			name:    "successful end vote",
			message: `/vote end abc123`,
			userId:  "user1",
			tarantoolMock: func() *MockTarantoolConnection {
				return &MockTarantoolConnection{
					SelectFunc: func(space, index interface{}, offset, limit uint32, iterator tarantool.Iter, key interface{}) (*tarantool.Response, error) {
						return &tarantool.Response{
							Data: [][]interface{}{
								{
									"abc123",
									"channel1",
									"user1", // creator
									"Test question?",
									map[interface{}]interface{}{"Option1": 5, "Option2": 3},
									int64(1234567890),
									true,
									map[string]bool{},
								},
							},
						}, nil
					},
					CallFunc: func(functionName string, args interface{}) (*tarantool.Response, error) {
						return &tarantool.Response{}, nil
					},
				}
			},
		},
		{
			name:    "not creator",
			message: `/vote end abc123`,
			userId:  "user2",
			tarantoolMock: func() *MockTarantoolConnection {
				return &MockTarantoolConnection{
					SelectFunc: func(space, index interface{}, offset, limit uint32, iterator tarantool.Iter, key interface{}) (*tarantool.Response, error) {
						return &tarantool.Response{
							Data: [][]interface{}{
								{
									"abc123",
									"channel1",
									"user1", // creator is user1
									"Test question?",
									map[interface{}]interface{}{"Option1": 5, "Option2": 3},
									int64(1234567890),
									true,
									map[string]bool{},
								},
							},
						}, nil
					},
				}
			},
			expectedError: "Только создатель может завершить голосование",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			post := &model.Post{
				Message:   tt.message,
				ChannelId: "test-channel",
				UserId:    tt.userId,
			}

			bot := &Bot{
				tarantool: tt.tarantoolMock(),
				client: &MockMattermostClient{
					CreatePostFunc: func(post *model.Post) (*model.Post, *model.Response, error) {
						if tt.expectedError != "" && post.Message == tt.expectedError {
							return nil, nil, nil
						}
						return nil, nil, nil
					},
				},
				user: &model.User{Id: "bot-user"},
			}

			bot.handleEndVote(post)
		})
	}
}

func TestHandleDeleteVote(t *testing.T) {
	tests := []struct {
		name          string
		message       string
		tarantoolMock func() *MockTarantoolConnection
		expectedError string
	}{
		{
			name:    "successful delete",
			message: `/vote delete abc123`,
			tarantoolMock: func() *MockTarantoolConnection {
				return &MockTarantoolConnection{
					CallFunc: func(functionName string, args interface{}) (*tarantool.Response, error) {
						return &tarantool.Response{}, nil
					},
				}
			},
		},
		{
			name:    "delete error",
			message: `/vote delete abc123`,
			tarantoolMock: func() *MockTarantoolConnection {
				return &MockTarantoolConnection{
					CallFunc: func(functionName string, args interface{}) (*tarantool.Response, error) {
						return nil, errors.New("tarantool error")
					},
				}
			},
			expectedError: "Ошибка при удалении: tarantool error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			post := &model.Post{
				Message:   tt.message,
				ChannelId: "test-channel",
				UserId:    "user1",
			}

			bot := &Bot{
				tarantool: tt.tarantoolMock(),
				client: &MockMattermostClient{
					CreatePostFunc: func(post *model.Post) (*model.Post, *model.Response, error) {
						if tt.expectedError != "" && post.Message == tt.expectedError {
							return nil, nil, nil
						}
						return nil, nil, nil
					},
				},
				user: &model.User{Id: "bot-user"},
			}

			bot.handleDeleteVote(post)
		})
	}
}

func TestSendHelp(t *testing.T) {
	bot := &Bot{
		client: &MockMattermostClient{
			CreatePostFunc: func(post *model.Post) (*model.Post, *model.Response, error) {
				if !strings.Contains(post.Message, "Доступные команды:") {
					t.Errorf("Expected help message, got: %s", post.Message)
				}
				return nil, nil, nil
			},
		},
	}

	bot.sendHelp("test-channel")
}
