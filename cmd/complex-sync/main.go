package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"wilayah-go/pkg/config"
	"wilayah-go/pkg/db"
	"wilayah-go/pkg/sync"

	"github.com/robfig/cron/v3"
)

func main() {
	cfg, err := config.LoadConfig(config.ModeComplex)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func(database *sql.DB) {
		err := database.Close()
		if err != nil {

		}
	}(database)

	fmt.Println("Starting Complex Area Sync Service...")

	// Run initial sync
	if err := sync.SynchronizeData(database, cfg); err != nil {
		log.Printf("Initial sync failed: %v", err)
	}

	c := cron.New()
	_, err = c.AddFunc(cfg.CronSchedule, func() {
		fmt.Println("Running scheduled complex sync...")
		if err := sync.SynchronizeData(database, cfg); err != nil {
			log.Printf("Scheduled complex sync failed: %v", err)
		}
	})
	if err != nil {
		log.Fatalf("Failed to setup cron: %v", err)
	}

	c.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("Shutting down complex sync service...")
	c.Stop()
}
