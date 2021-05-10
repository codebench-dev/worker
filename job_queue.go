package main

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

type jobQueue struct {
	redis *redis.Client
}

func (q jobQueue) getJob(ctx context.Context) ([]string, error) {
	result, err := q.redis.BLPop(ctx, 0*time.Second, "jobs").Result()

	return result, err
}

func (q jobQueue) setjobReceived(ctx context.Context, job benchJob) (int64, error) {
	result, err := q.redis.RPush(ctx, job.Id, "received").Result()

	if err != nil {
		log.WithError(err).Error("Failed to set job status in queue")
	}
	if result < 1 {
		log.Error("Failed to set job status in queue")
	}

	return result, err
}

func (q jobQueue) setjobRunning(ctx context.Context, job benchJob) (int64, error) {
	result, err := q.redis.RPush(ctx, job.Id, "running").Result()

	if err != nil {
		log.WithError(err).Error("Failed to set job status in queue")
	}
	if result < 1 {
		log.Error("Failed to set job status in queue")
	}

	return result, err
}

func (q jobQueue) setjobFailed(ctx context.Context, job benchJob) (int64, error) {
	result, err := q.redis.RPush(ctx, job.Id, "failed").Result()

	if err != nil {
		log.WithError(err).Error("Failed to set job status in queue")
	}
	if result < 1 {
		log.Error("Failed to set job status in queue")
	}

	return result, err
}

func (q jobQueue) setjobResult(ctx context.Context, job benchJob, res agentExecRes) (int64, error) {
	result, err := q.redis.RPush(ctx, job.Id, "done", res.StdOut, res.StdErr).Result()

	if err != nil {
		log.WithError(err).Error("Failed to set job status in queue")
	}
	if result < 1 {
		log.Error("Failed to set job status in queue")
	}

	return result, err
}
