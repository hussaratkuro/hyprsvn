package main

import (
	"log"
	"os"

	"gioui.org/app"

	"github.com/hussaratkuro/hyprsvn/internal/svn"
	"github.com/hussaratkuro/hyprsvn/internal/ui"
)

func main() {
	configs := svn.LoadRepoConfigs()
	go func() {
		if err := ui.Run(configs); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}
