package main

import (
	"log/slog"
	appcontext "meteo/context"
	"meteo/data"
	"meteo/screens"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func createLogger() *slog.Logger {
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	return slog.New(slog.NewJSONHandler(file, nil))
}

func main() {
	logger := createLogger()
	logger.Info("Démarrage de l'App")

	db, err := data.InitDB()
	if err != nil {
		logger.Error("Can't init database", "error", err)
		panic(err)
	}

	a := app.New()
	w := a.NewWindow("Météo")
	appcontext.SetAppContext(w, db, logger)
	w.Resize(fyne.NewSize(500, 500))

	screen := screens.InitHomeScreen()
	w.SetContent(screen.Render())

	screen.LoadExistingData()

	w.ShowAndRun()
}
