package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/adrg/xdg"
	"github.com/wolf29f/hushtongue/internal/services"
	"github.com/wolf29f/hushtongue/internal/services/storage/sqlite"
	"github.com/wolf29f/hushtongue/internal/tui"
	"github.com/wolf29f/hushtongue/internal/tui/startup"
)

func main() {
	logFile, err := setupLogger()
	if err != nil {
		log.Fatalf("Failed to set up logger: %v", err)
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			log.Printf("Failed to close log file: %v", err)
		}
	}()

	db, err := sqlite.Load("")
	if err != nil {
		fmt.Printf("Failed to load SQLite: %v", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Warn("Failed to close database", "error", err)
		}
	}()

	services := services.New(db)

	p := tea.NewProgram(tui.NewRootModel(startup.NewModel(services)))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func setupLogger() (*os.File, error) {
	logPath, err := xdg.StateFile("hushtongue/app.log")
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})

	slog.SetDefault(slog.New(handler))
	return f, nil
}
