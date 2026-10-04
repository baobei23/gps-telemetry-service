package main

import (
	"fmt"

	"github.com/baobei23/goapp/internal/configs"
	"github.com/baobei23/goapp/internal/pkg/postgres"
)

func main() {
	cfgs, err := configs.New()
	if err != nil {
		panic(fmt.Errorf("failed to load configurations: %w", err))
	}

	pgConfig := cfgs.Postgres()
	pgConfig.EnableTracing = false

	pqdriver, err := postgres.NewPool(pgConfig)
	if err != nil {
		panic(fmt.Errorf("failed to connect to postgres: %w", err))
	}
	defer pqdriver.Close()
	fmt.Println("Successfully connected to database!")
}
