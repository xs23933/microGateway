package main

import (
	"github.com/xs23933/microgateway"
)

func main() {
	conf := microgateway.LoadConfigFile("config.yml")
	app := microgateway.New(conf)
	if err := app.Start(); err != nil {
		panic(err)
	}
}
