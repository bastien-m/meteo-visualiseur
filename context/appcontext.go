package appcontext

import (
	"database/sql"
	"log/slog"
	"sync"

	"fyne.io/fyne/v2"
)

type appContext struct {
	W      fyne.Window
	DB     *sql.DB
	Logger *slog.Logger
}

var instance *appContext
var once sync.Once

func SetAppContext(w fyne.Window, db *sql.DB, logger *slog.Logger) {
	once.Do(func() {
		instance = &appContext{
			W:      w,
			DB:     db,
			Logger: logger,
		}
	})
}

func GetAppContext() appContext {
	if instance == nil {
		panic("Error: instance is nil. Should call SetDialogContext first")
	}
	return *instance
}
