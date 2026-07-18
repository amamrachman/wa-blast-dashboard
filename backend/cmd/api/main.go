package main

import (
	"backend/pkg/whatsapp"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

type BlastPayload struct {
	Numbers   []string `json:"numbers"`
	Message   string   `json:"message"`
	ImageData string   `json:"image_data"`
}

func main() {

	waEngine := whatsapp.NewWAEngine()

	go waEngine.Connect()

	app := fiber.New(fiber.Config{
		BodyLimit: 10 * 1024 * 1024,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	app.Post("/api/blast", func(c *fiber.Ctx) error {
		var payload BlastPayload
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Format JSON tidak valid"})
		}

		if waEngine.Client == nil || !waEngine.Client.IsConnected() {
			return c.Status(500).JSON(fiber.Map{"error": "Aplikasi belum terhubung ke WhatsApp"})
		}

		go func() {
			fmt.Printf("\n🚀 Memulai Blast ke %d nomor...\n", len(payload.Numbers))

			for i, number := range payload.Numbers {
				err := waEngine.SendBlastMessage(number, payload.Message, payload.ImageData)
				if err != nil {
					fmt.Printf("[%d] ❌ Gagal kirim ke %s: %v\n", i+1, number, err)
				} else {
					fmt.Printf("[%d] ✅ Berhasil kirim ke %s\n", i+1, number)
				}

				time.Sleep(5 * time.Second)
			}
			fmt.Println("🎉 Semua proses blast selesai dilakukan!")
		}()

		return c.JSON(fiber.Map{
			"message": "Proses blast berhasil dimulai di background.",
			"target":  len(payload.Numbers),
		})
	})

	log.Fatal(app.Listen(":3000"))
}
