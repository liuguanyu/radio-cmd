package main

import (
	"log"

	"github.com/liuguanyu/radio-cmd/pkg/tui"
)

func main() {
	app := tui.NewApp()
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}