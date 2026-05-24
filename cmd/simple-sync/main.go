package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"wilayah-go/pkg/config"
	"wilayah-go/pkg/db"
	"wilayah-go/pkg/sync"

	"github.com/robfig/cron/v3"
)

func main() {
	force := false
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--force") {
			force = true
			break
		}
	}

	cfg, err := config.LoadConfig(config.ModeSimple, force)
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

	fmt.Println("Starting Indonesia Regions Simple Sync Service...")

	// Run initial sync
	if err := sync.SynchronizeData(database, cfg); err != nil {
		log.Printf("Initial sync failed: %v", err)
	}

	c := cron.New()
	_, err = c.AddFunc(cfg.CronSchedule, func() {
		fmt.Println("Running scheduled simple sync...")
		if err := sync.SynchronizeData(database, cfg); err != nil {
			log.Printf("Scheduled simple sync failed: %v", err)
		}
	})
	if err != nil {
		log.Fatalf("Failed to setup cron: %v", err)
	}

	c.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("Shutting down simple sync service...")
	c.Stop()
}
