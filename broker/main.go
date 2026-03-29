// Copyright 2026 Sam Schreiber
//
// This file is part of nito.
//
// nito is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// nito is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with nito. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/srschreiber/nito/broker/database"
	"github.com/srschreiber/nito/broker/handler"
	brokerws "github.com/srschreiber/nito/broker/websocket"
)

var configPath = flag.String("config", "broker/config.yml", "path to config file")
var migrateOnly = flag.Bool("migrate-only", false, "run migrations and exit")

func main() {
	flag.Parse()
	log.SetFlags(0)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Single connection used only for sequential migration at startup.
	conn, err := database.NewPostgres(cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	if err := database.RunMigrations(conn); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	conn.Close(context.Background())

	if *migrateOnly {
		return
	}

	// Pool used by the broker for concurrent request handling.
	pool, err := database.NewPool(cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)
	if err != nil {
		log.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	broker := brokerws.NewBroker(ctx, cfg.Broker.Addr, pool)
	handler.New(broker, pool).Register()

	log.Printf("broker listening on %s", cfg.Broker.Addr)
	log.Fatal(http.ListenAndServe(cfg.Broker.Addr, nil))
}
