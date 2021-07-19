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
	ID           string `json:"id"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	Error        string `json:"error"`
	StdErr       string `json:"stderr"`
	StdOut       string `json:"stdout"`
	ExecDuration int    `json:"exec_duration"`
	MemUsage     int64  `json:"mem_usage"`
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

	err = ch.ExchangeDeclare(
		"jobs_ex", // name
		"direct",  // type
		true,      // durable
		false,     // auto-deleted
		false,     // internal
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to declare an exchange")
	}

	jobsQ, err := ch.QueueDeclare(
		"jobs_q", // name
		true,     // durable
		false,    // delete when unused
		false,    // exclusive
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to declare a queue")
	}

	err = ch.QueueBind(
		jobsQ.Name, // queue name
		"jobs_rk",  // routing key
		"jobs_ex",  // exchange
		false,
		nil)
	if err != nil {
		log.WithError(err).Fatal("Failed to bind a queue")
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

func (q jobQueue) getQueueForJob(ctx context.Context) error {
	return q.ch.ExchangeDeclare(
		"jobs_status_ex", // name
		"direct",         // type
		false,            // durable
		false,            // auto-deleted
		false,            // internal
		false,            // no-wait
		nil,              // arguments
	)
}

func (q jobQueue) setjobStatus(ctx context.Context, job benchJob, status string, res agentExecRes) error {
	log.WithField("status", status).Info("Set job status")
	jobStatus := &jobStatus{
		ID:      job.ID,
		Status:  status,
		Message: res.Message,
		Error:   res.Error,
		StdErr:  "",
		StdOut:  "",
	}
	b, err := json.Marshal(jobStatus)
	if err != nil {
		return err
	}
	err = q.ch.Publish(
		"jobs_status_ex", // exchange
		"jobs_status_rk", // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        b,
		})
	return err
}

func (q jobQueue) setjobReceived(ctx context.Context, job benchJob) error {
	return q.setjobStatus(ctx, job, "received", agentExecRes{})
}

func (q jobQueue) setjobRunning(ctx context.Context, job benchJob) error {
	return q.setjobStatus(ctx, job, "running", agentExecRes{})
}

func (q jobQueue) setjobFailed(ctx context.Context, job benchJob, res agentExecRes) error {
	return q.setjobStatus(ctx, job, "failed", res)
}
func (q jobQueue) setjobResult(ctx context.Context, job benchJob, res agentExecRes) error {
	jobStatus := &jobStatus{
		ID:           job.ID,
		Status:       "done",
		Message:      res.Message,
		Error:        res.Error,
		StdErr:       res.StdErr,
		StdOut:       res.StdOut,
		ExecDuration: res.ExecDuration,
		MemUsage:     res.MemUsage,
	}
	log.WithField("jobStatus", jobStatus).Info("Set job result")

	b, err := json.Marshal(jobStatus)
	if err != nil {
		return err
	}
	err = q.ch.Publish(
		"jobs_status_ex", // exchange
		"jobs_status_rk", // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        b,
		})
	return err
}
