package main

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

type jobQueueRedis struct {
	redis *redis.Client
}

func (q jobQueueRedis) getJob(ctx context.Context) ([]string, error) {
	result, err := q.redis.BLPop(ctx, 0*time.Second, "jobs").Result()

	return result, err
}

func (q jobQueueRedis) setjobReceived(ctx context.Context, job benchJob) (int64, error) {
	result, err := q.redis.RPush(ctx, job.ID, "received").Result()

	if err != nil {
		log.WithError(err).Error("Failed to set job status in queue")
	}
	if result < 1 {
		log.Error("Failed to set job status in queue")
	}

	return result, err
}

func (q jobQueueRedis) setjobRunning(ctx context.Context, job benchJob) (int64, error) {
	result, err := q.redis.RPush(ctx, job.ID, "running").Result()

	if err != nil {
		log.WithError(err).Error("Failed to set job status in queue")
	}
	if result < 1 {
		log.Error("Failed to set job status in queue")
	}

	return result, err
}

func (q jobQueueRedis) setjobFailed(ctx context.Context, job benchJob) (int64, error) {
	result, err := q.redis.RPush(ctx, job.ID, "failed").Result()

	if err != nil {
		log.WithError(err).Error("Failed to set job status in queue")
	}
	if result < 1 {
		log.Error("Failed to set job status in queue")
	}

	return result, err
}

func (q jobQueueRedis) setjobResult(ctx context.Context, job benchJob, res agentExecRes) (int64, error) {
	result, err := q.redis.RPush(ctx, job.ID, "done", res.StdOut, res.StdErr).Result()

	if err != nil {
		log.WithError(err).Error("Failed to set job status in queue")
	}
	if result < 1 {
		log.Error("Failed to set job status in queue")
	}

	return result, err
}
