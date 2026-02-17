package main

import (
	"log/slog"
	"meteo/screens"
	"os"
)

func main() {
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	logger := slog.New(slog.NewJSONHandler(file, nil))

	logger.Info("Démarrage de l'App")

	screens.Run(logger)

}
