package helpers

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

var ctx = context.Background()

func SetRedis(key string, data interface{}, expire time.Duration) {
	dbHost := os.Getenv("DB_REDIS_HOST")
	dbPass := os.Getenv("DB_REDIS_PASSWORD")
	dbName, _ := strconv.Atoi(os.Getenv("DB_REDIS_NAME"))

	rdb := redis.NewClient(&redis.Options{
		Addr:     dbHost,
		Password: dbPass,
		DB:       dbName,
	})
	json, _ := json.Marshal(data)
	err := rdb.Set(ctx, key, json, expire).Err()
	rdb.Close()
	ErrorCheck(err)
}
func GetRedis(key string) (string, bool) {
	flag := true
	dbHost := os.Getenv("DB_REDIS_HOST")
	dbPass := os.Getenv("DB_REDIS_PASSWORD")
	dbName, _ := strconv.Atoi(os.Getenv("DB_REDIS_NAME"))

	rdb := redis.NewClient(&redis.Options{
		Addr:     dbHost,
		Password: dbPass,
		DB:       dbName,
	})

	resultredis, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		flag = false
	}
	rdb.Close()
	return resultredis, flag
}
func DeleteRedis(key string) int {
	dbHost := os.Getenv("DB_REDIS_HOST")
	dbPass := os.Getenv("DB_REDIS_PASSWORD")
	dbName, _ := strconv.Atoi(os.Getenv("DB_REDIS_NAME"))

	rdb := redis.NewClient(&redis.Options{
		Addr:     dbHost,
		Password: dbPass,
		DB:       dbName,
	})
	return int(rdb.Del(ctx, key).Val())
}
