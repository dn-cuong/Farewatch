package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/farewatch/farewatch/internal/models"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
	ttl    time.Duration
	hits   atomic.Int64
	misses atomic.Int64
}

func New(redisURL string, ttl time.Duration) (*Cache, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &Cache{client: client, ttl: ttl}, nil
}

func (c *Cache) Close() error { return c.client.Close() }

func fareKey(routeID, airlineCode string) string {
	return fmt.Sprintf("fare:%s:%s", routeID, airlineCode)
}

func (c *Cache) GetFare(ctx context.Context, routeID, airlineCode string) (*models.Fare, bool, error) {
	val, err := c.client.Get(ctx, fareKey(routeID, airlineCode)).Result()
	if err == redis.Nil {
		c.misses.Add(1)
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var fare models.Fare
	if err := json.Unmarshal([]byte(val), &fare); err != nil {
		c.misses.Add(1)
		return nil, false, nil
	}
	fare.Cached = true
	c.hits.Add(1)
	return &fare, true, nil
}

func (c *Cache) SetFare(ctx context.Context, fare models.Fare) error {
	b, err := json.Marshal(fare)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, fareKey(fare.RouteID, fare.AirlineCode), b, c.ttl).Err()
}

func (c *Cache) Stats() (hits, misses int64, hitRate float64) {
	hits, misses = c.hits.Load(), c.misses.Load()
	total := hits + misses
	if total == 0 {
		return hits, misses, 0
	}
	return hits, misses, float64(hits) / float64(total) * 100
}

func (c *Cache) ResetStats() {
	c.hits.Store(0)
	c.misses.Store(0)
}
