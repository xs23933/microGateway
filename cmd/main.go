package main

import (
	"core"
)

func main() {
	conf := core.LoadConfigFile("config.yml")
	app := core.New(conf)
	if err := app.Start(); err != nil {
		panic(err)
	}
}
