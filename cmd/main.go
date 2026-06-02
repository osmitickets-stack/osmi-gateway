package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/franciscozamorau/osmi-gateway/internal/config"
	"github.com/franciscozamorau/osmi-gateway/internal/server"
	"github.com/joho/godotenv" // 🔥 AGREGAR ESTE IMPORT
)

func main() {
	// 🔥 CARGAR .env ANTES DE TODO (IGUAL QUE EL SERVER)
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ No .env file found, using system environment variables")
	}

	// 1. Cargar configuración
	cfg := config.Load()

	// 2. Crear servidor
	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Error al crear servidor: %v", err)
	}

	// 3. Manejo de señales para shutdown graceful
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("🛑 Apagando servidor...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Stop(ctx); err != nil {
			log.Printf("❌ Error al apagar: %v", err)
		} else {
			log.Println("✅ Servidor apagado correctamente")
		}
	}()

	// 4. Iniciar servidor
	log.Printf("🚀 Gateway iniciado en puerto %s", cfg.HTTPPort)
	if err := srv.Start(); err != nil {
		log.Fatalf("❌ Error en servidor: %v", err)
	}
}
