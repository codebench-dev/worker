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

func (q jobQueue) getJob() ([]string, error) {
	result, err := q.redis.BLPop(context.Background(), 0*time.Second, "jobs").Result()

	return result, err
}

func (q jobQueue) setjobReceived(job benchJob) (int64, error) {
	result, err := q.redis.RPush(context.Background(), job.Id, "received").Result()

	if err != nil {
		log.WithError(err).Error("Failed to set job status in queue")
	}
	if result < 1 {
		log.Error("Failed to set job status in queue")
	}

	return result, err
}

func (q jobQueue) setjobRunning(job benchJob) (int64, error) {
	result, err := q.redis.RPush(context.Background(), job.Id, "running").Result()

	if err != nil {
		log.WithError(err).Error("Failed to set job status in queue")
	}
	if result < 1 {
		log.Error("Failed to set job status in queue")
	}

	return result, err
}

func (q jobQueue) setjobFailed(job benchJob) (int64, error) {
	result, err := q.redis.RPush(context.Background(), job.Id, "failed").Result()

	if err != nil {
		log.WithError(err).Error("Failed to set job status in queue")
	}
	if result < 1 {
		log.Error("Failed to set job status in queue")
	}

	return result, err
}

func (q jobQueue) setjobResult(job benchJob, res agentExecRes) (int64, error) {
	result, err := q.redis.RPush(context.Background(), job.Id, "done", res.StdOut, res.StdErr).Result()

	if err != nil {
		log.WithError(err).Error("Failed to set job status in queue")
	}
	if result < 1 {
		log.Error("Failed to set job status in queue")
	}

	return result, err
}
