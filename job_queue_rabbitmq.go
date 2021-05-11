package main

import (
	"context"
	"encoding/json"

	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

type jobQueue struct {
	ch    *amqp.Channel
	conn  *amqp.Connection
	jobsQ amqp.Queue
	jobs  <-chan amqp.Delivery
}

type jobStatus struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Command string `json:"command"`
	StdErr  string `json:"stderr"`
	StdOut  string `json:"stdout"`
}

func newJobQueue(endpoint string) jobQueue {
	conn, err := amqp.Dial(endpoint)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to RabbitMQ")
	}

	ch, err := conn.Channel()
	if err != nil {
		log.WithError(err).Fatal("Failed to open a channel")
	}

	jobsQ, err := ch.QueueDeclare(
		"jobs", // name
		true,   // durable
		false,  // delete when unused
		false,  // exclusive
		false,  // no-wait
		nil,    // arguments
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to declare a queue")
	}

	jobs, err := ch.Consume(
		jobsQ.Name, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to register a consumer")
	}

	return jobQueue{
		ch,
		conn,
		jobsQ,
		jobs,
	}
}

func (q jobQueue) getQueueForJob(ctx context.Context, job benchJob) error {
	return q.ch.ExchangeDeclare(
		"job_status", // name
		"direct",     // type
		false,        // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
}

func (q jobQueue) setjobStatus(ctx context.Context, job benchJob, status string) error {
	log.WithField("status", status).Info("Set job status")
	jobStatus := &jobStatus{
		ID:      job.ID,
		Status:  status,
		Command: job.Command,
		StdErr:  "",
		StdOut:  "",
	}
	b, err := json.Marshal(jobStatus)
	if err != nil {
		return err
	}
	err = q.ch.Publish(
		"job_status", // exchange
		job.ID,       // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        b,
		})
	return err
}

func (q jobQueue) setjobReceived(ctx context.Context, job benchJob) error {
	return q.setjobStatus(ctx, job, "received")
}

func (q jobQueue) setjobRunning(ctx context.Context, job benchJob) error {
	return q.setjobStatus(ctx, job, "running")
}

func (q jobQueue) setjobFailed(ctx context.Context, job benchJob) error {
	return q.setjobStatus(ctx, job, "failed")
}
func (q jobQueue) setjobResult(ctx context.Context, job benchJob, res agentExecRes) error {
	jobStatus := &jobStatus{
		ID:      job.ID,
		Status:  "done",
		Command: job.Command,
		StdErr:  res.StdErr,
		StdOut:  res.StdOut,
	}
	log.WithField("jobStatus", jobStatus).Info("Set job result")

	b, err := json.Marshal(jobStatus)
	if err != nil {
		return err
	}
	err = q.ch.Publish(
		"job_status", // exchange
		job.ID,       // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        b,
		})
	return err
}
