package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"github.com/go-redis/redis/v8"
)

var rdb *redis.Client

func initRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.RedisHost, config.RedisPort),
		Password: config.RedisPass,
		DB:       0,
	})
}

func getConversationHistory(ctx context.Context, userID int64) ([]Message, error) {
	key := fmt.Sprintf("conversation:%d", userID)
	data, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return []Message{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get error: %w", err)
	}

	var messages []Message
	if err := json.Unmarshal([]byte(data), &messages); err != nil {
		return nil, fmt.Errorf("json unmarshal error: %w", err)
	}
	return messages, nil
}

func saveConversationHistory(ctx context.Context, userID int64, messages []Message) error {
	key := fmt.Sprintf("conversation:%d", userID)
	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}
	return rdb.Set(ctx, key, string(data), 0).Err()
}

func clearConversationHistory(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("conversation:%d", userID)
	return rdb.Del(ctx, key).Err()
}

func getUserModel(ctx context.Context, userID int64) (string, error) {
	key := fmt.Sprintf("user:%d:model", userID)
	model, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		// If no model is set, return default model from config
		return config.OpenRouterModel, nil
	}
	if err != nil {
		return "", fmt.Errorf("redis get error: %w", err)
	}
	return model, nil
}

func setUserModel(ctx context.Context, userID int64, model string) error {
	key := fmt.Sprintf("user:%d:model", userID)
	return rdb.Set(ctx, key, model, 0).Err()
}

func getUserMode(ctx context.Context, userID int64) (string, error) {
	key := fmt.Sprintf("user:%d:mode", userID)
	mode, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		// If no mode is set, return default mode as text
		return "text", nil
	}
	if err != nil {
		return "", fmt.Errorf("redis get error: %w", err)
	}
	return mode, nil
}

func setUserMode(ctx context.Context, userID int64, mode string) error {
	key := fmt.Sprintf("user:%d:mode", userID)
	return rdb.Set(ctx, key, mode, 0).Err()
}

func saveUserImage(ctx context.Context, userID int64, fileID string, prompt string) error {
	key := fmt.Sprintf("user:%d:images", userID)
	imageData := map[string]string{
		"file_id": fileID,
		"prompt":  prompt,
		"date":    time.Now().Format(time.RFC3339),
	}
	jsonData, err := json.Marshal(imageData)
	if err != nil {
		return fmt.Errorf("failed to marshal image data: %w", err)
	}
	return rdb.RPush(ctx, key, string(jsonData)).Err()
}

type UserImage struct {
	FileID string `json:"file_id"`
	Prompt string `json:"prompt"`
	Date   string `json:"date"`
}

func getUserImages(ctx context.Context, userID int64) ([]UserImage, error) {
	key := fmt.Sprintf("user:%d:images", userID)
	data, err := rdb.LRange(ctx, key, 0, -1).Result()
	if err == redis.Nil {
		return []UserImage{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get error: %w", err)
	}

	var images []UserImage
	for _, item := range data {
		var image UserImage
		if err := json.Unmarshal([]byte(item), &image); err != nil {
			return nil, fmt.Errorf("json unmarshal error: %w", err)
		}
		images = append(images, image)
	}
	return images, nil
}
