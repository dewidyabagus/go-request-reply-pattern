package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dewidyabagus/go-request-reply-pattern/model"
	"github.com/dewidyabagus/go-request-reply-pattern/utils"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server service",
	Run:   func(_ *cobra.Command, _ []string) { RunServerService() },
}

var (
	random *rand.Rand
	r      = strings.NewReplacer("\n", "", "\r", "", "\t", "")
)

func init() {
	random = rand.New(rand.NewSource(time.Now().UnixMilli()))
}

func RunServerService() {
	signal, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)

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

	ch, err := conn.Channel()
	utils.FailOnError(err, "Open New Channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		os.Getenv("RMQ_QUEUE_NAME"), true, false, false, false,
		amqp.Table{
			"x-queue-type": amqp.QueueTypeQuorum,
		},
	)
	utils.FailOnError(err, "Queue Declare")

	err = ch.Qos(1, 0, false)
	utils.FailOnError(err, "Qos Setting")

	wg := new(sync.WaitGroup)

	for range 3 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			ct, err := conn.Channel()
			utils.FailOnError(err, "Open Channel Thread")
			defer ct.Close()

			msgs, err := ct.ConsumeWithContext(context.Background(), q.Name, "", false, false, false, false, nil)
			utils.FailOnError(err, "Consume Message: "+q.Name)

			for {
				select {
				case <-signal.Done():
					return

				case msg := <-msgs:
					payload := model.Payload{}
					_ = json.Unmarshal(msg.Body, &payload)

					st := time.Now()
					log.Println("CorrelationId:", msg.CorrelationId, "Body:", r.Replace(string(msg.Body)), "Reply-to:", msg.ReplyTo)

					randomSleep()

					response := model.ReplyPayload{
						Source:        "Server",
						CorrelationId: msg.CorrelationId,
						Duration:      time.Since(st).String(),
						Payload:       payload,
					}
					rawResponse, _ := json.Marshal(response)

					if msg.ReplyTo != "" {
						err = ch.Publish(
							"",          // Using Default Exchange
							msg.ReplyTo, // Queue Name
							false,       // Mandatory
							false,       // Immediate
							amqp.Publishing{
								ContentType:   msg.ContentType,
								CorrelationId: msg.CorrelationId,
								Body:          rawResponse,
							},
						)
						if err != nil {
							log.Println("[ERROR] Publish reply message:", err.Error())
						}
					}
					msg.Ack(false)
				}
			}
		}()
	}

	log.Println("Consumer server service ready")

	<-signal.Done()

	stop()

	wg.Wait()
}

func randomSleep() {
	time.Sleep(time.Duration(random.Int31n(400)) * time.Millisecond)
}
