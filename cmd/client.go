package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dewidyabagus/go-request-reply-pattern/model"
	"github.com/dewidyabagus/go-request-reply-pattern/utils"
	"github.com/google/uuid"

	chi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/spf13/cobra"
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Client service",
	Run:   func(_ *cobra.Command, _ []string) { RunClientService() },
}

const pseudoQueueReplyTo = "amq.rabbitmq.reply-to"

func RunClientService() {
	signal, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	config := amqp.Config{
		Heartbeat:  10 * time.Second,
		Properties: amqp.NewConnectionProperties(),
	}
	url := fmt.Sprintf(
		"amqp://%s:%s@%s:%s/",
		os.Getenv("RMQ_USERNAME"), os.Getenv("RMQ_PASSWORD"), os.Getenv("RMQ_HOST"), os.Getenv("RMQ_PORT"),
	)

	conn, err := amqp.DialConfig(url, config)
	utils.FailOnError(err, "Dial Connection")
	defer conn.Close()

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Post("/publish", func(w http.ResponseWriter, r *http.Request) {

		payload := model.Payload{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			utils.ResponseErrorJSON(w, http.StatusBadRequest, err)
			return
		}

		correlationId := uuid.NewString()
		rawBody, _ := json.Marshal(payload)

		log.Println("Incomming Request-> CorrelationId:", correlationId, "Body:", string(rawBody))

		ch, err := conn.Channel()
		if err != nil {
			utils.ResponseErrorJSON(w, http.StatusInternalServerError, err)
			return
		}
		defer ch.Close()

		msgs, err := ch.Consume(
			pseudoQueueReplyTo, "", true, true, false, false, nil,
		)
		if err != nil {
			utils.ResponseErrorJSON(w, http.StatusInternalServerError, err)
			return
		}

		// Publish using the default exchange
		err = ch.Publish(
			"", os.Getenv("RMQ_QUEUE_NAME"), false, false, amqp.Publishing{
				Body:          rawBody,
				CorrelationId: correlationId,
				ContentType:   "application/json",
				ReplyTo:       pseudoQueueReplyTo,
			},
		)
		if err != nil {
			utils.ResponseErrorJSON(w, http.StatusInternalServerError, err)
			return
		}

		to, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		select {
		case <-to.Done():
			utils.ResponseErrorJSON(w, http.StatusRequestTimeout, errors.New("request timeout"))

		case msg := <-msgs:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(msg.Body)
		}
	})

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8000"
	}
	server := &http.Server{
		Handler:      r,
		Addr:         ":" + port,
		WriteTimeout: 90 * time.Second, ReadTimeout: 90 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalln("HTTP Server:", err.Error())
		}
	}()

	log.Println("HTTP service runs on", server.Addr)

	<-signal.Done()

	stop()

	if err := server.Shutdown(context.Background()); err != nil {
		log.Println("[error] Shutdown http server")
	}
}
