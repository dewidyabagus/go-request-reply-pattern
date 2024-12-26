package main

import (
	"log"

	"github.com/dewidyabagus/go-request-reply-pattern/cmd"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("./.env"); err != nil {
		log.Panicln("Load Env:", err.Error())
	}
	_ = cmd.Execute()
}
