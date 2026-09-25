package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"backend/pkg/whatsapp"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/robfig/cron/v3"
	"go.mau.fi/whatsmeow"
)

type BlastPayload struct {
	Numbers     []string `json:"numbers"`
	Message     string   `json:"message"`
	ImageData   string   `json:"image_data"`
	ScheduledAt string   `json:"scheduled_at"`
}

func sendDelay() time.Duration {
    jitter := time.Duration(rand.Intn(10000)) * time.Millisecond
    return 15*time.Second + jitter
}

func runBlast(waEngine *whatsapp.WAEngine, numbers []string, message string, imageData string, logPrefix string) {
	waEngine.MarkSending()
	defer waEngine.MarkDone()

	var uploadedMedia *whatsmeow.UploadResponse
	var fileBytes []byte
	var mimeType string

	if imageData != "" {
		up, bytes, mime, err := waEngine.PrepareImage(imageData)
		if err != nil {
			fmt.Printf("❌ %s Gagal upload gambar: %v\n", logPrefix, err)
		} else {
			uploadedMedia = &up
			fileBytes = bytes
			mimeType = mime
		}
	}

	sent, failed := 0, 0
	for i, number := range numbers {
		if waEngine.Client == nil || !waEngine.Client.IsConnected() {
			remaining := len(numbers) - i
			fmt.Printf("⚠️ %s Koneksi WhatsApp terputus. Menghentikan proses, %d nomor sisanya dilewati.\n", logPrefix, remaining)
			failed += remaining
			break
		}

		err := waEngine.SendPreparedMessage(number, message, uploadedMedia, fileBytes, mimeType)
		if err != nil {
			fmt.Printf("[%d/%d] ❌ %s Gagal kirim ke %s: %v\n", i+1, len(numbers), logPrefix, number, err)
			failed++
		} else {
			fmt.Printf("[%d/%d] ✅ %s Berhasil kirim ke %s\n", i+1, len(numbers), logPrefix, number)
			sent++
		}

		if i < len(numbers)-1 {
			time.Sleep(sendDelay())
		}
	}

	fmt.Printf("🎉 %s Selesai. Terkirim: %d, Gagal: %d\n", logPrefix, sent, failed)
}

func startSchedulerWorker(waEngine *whatsapp.WAEngine) *cron.Cron {
	c := cron.New()

	c.AddFunc("* * * * *", func() {
		now := time.Now()

		query := `SELECT id, numbers, message, image_data FROM scheduled_blasts WHERE status = 'PENDING' AND scheduled_at <= ?`
		rows, err := waEngine.DB.Query(query, now)
		if err != nil {
			fmt.Printf("⚠️ [SCHEDULER] Gagal query jadwal: %v\n", err)
			return
		}

		type job struct {
			id                   int
			rawNumbers, msg, img string
		}
		var jobs []job

		for rows.Next() {
			var j job
			if err := rows.Scan(&j.id, &j.rawNumbers, &j.msg, &j.img); err != nil {
				continue
			}
			jobs = append(jobs, j)
		}
		if err := rows.Err(); err != nil {
			fmt.Printf("⚠️ Error saat membaca baris data scheduled_blasts: %v\n", err)
		}
		rows.Close()

		for _, j := range jobs {
			res, err := waEngine.DB.Exec(`UPDATE scheduled_blasts SET status = 'PROCESSING' WHERE id = ? AND status = 'PENDING'`, j.id)
			if err != nil {
				continue
			}
			if n, _ := res.RowsAffected(); n == 0 {

				continue
			}

			go func(blastID int, numStr, msg, img string) {
				numbers := strings.Split(numStr, ",")
				fmt.Printf("\n⏰ [SCHEDULER] Memulai eksekusi jadwal #%d ke %d nomor...\n", blastID, len(numbers))

				runBlast(waEngine, numbers, msg, img, fmt.Sprintf("[JADWAL #%d]", blastID))

				waEngine.DB.Exec(`UPDATE scheduled_blasts SET status = 'COMPLETED' WHERE id = ?`, blastID)
				fmt.Printf("🎉 [SCHEDULER] Jadwal #%d selesai!\n", blastID)
			}(j.id, j.rawNumbers, j.msg, j.img)
		}
	})

	c.Start()
	return c
}

func main() {
	waEngine := whatsapp.NewWAEngine()

	go waEngine.Connect()

	cronScheduler := startSchedulerWorker(waEngine)

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

		if len(payload.Numbers) == 0 || payload.Message == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Daftar nomor dan pesan wajib diisi"})
		}

		var invalidNumbers []string
		for _, n := range payload.Numbers {
			if _, err := whatsapp.NormalizeNumber(n); err != nil {
				invalidNumbers = append(invalidNumbers, n)
			}
		}
		if len(invalidNumbers) > 0 {
			return c.Status(400).JSON(fiber.Map{
				"error":           "Beberapa nomor tidak valid",
				"invalid_numbers": invalidNumbers,
			})
		}

		if waEngine.Client == nil || !waEngine.Client.IsConnected() {
			return c.Status(500).JSON(fiber.Map{"error": "Aplikasi belum terhubung ke WhatsApp"})
		}

		if payload.ScheduledAt != "" {
			parsedTime, err := time.Parse(time.RFC3339, payload.ScheduledAt)
			if err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "Format scheduled_at harus ISO 8601 (contoh: 2026-07-30T10:00:00Z)"})
			}

			if parsedTime.Before(time.Now()) {
				return c.Status(400).JSON(fiber.Map{"error": "Waktu jadwal tidak boleh di masa lalu"})
			}

			err = waEngine.ScheduleBlastMessage(payload.Numbers, payload.Message, payload.ImageData, parsedTime)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Gagal menyimpan jadwal pesan ke database"})
			}

			return c.JSON(fiber.Map{
				"message":      "Pesan berhasil dijadwalkan. PENTING: proses backend ini harus tetap berjalan sampai waktu tersebut agar pesan benar-benar terkirim.",
				"target":       len(payload.Numbers),
				"scheduled_at": parsedTime.Format("2006-01-02 15:04:05"),
			})
		}

		numbersCopy := make([]string, len(payload.Numbers))
		copy(numbersCopy, payload.Numbers)

		go func() {
			fmt.Printf("\n🚀 Memulai Blast Langsung ke %d nomor...\n", len(numbersCopy))
			runBlast(waEngine, numbersCopy, payload.Message, payload.ImageData, "[LANGSUNG]")
		}()

		return c.JSON(fiber.Map{
			"message": "Proses blast berhasil dimulai di background.",
			"target":  len(payload.Numbers),
		})
	})

	go func() {
		if err := app.Listen(":3000"); err != nil {
			log.Printf("Server berhenti: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	fmt.Println("\n🛑 Sinyal shutdown diterima, mematikan aplikasi dengan aman...")

	cronScheduler.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Printf("Error saat shutdown Fiber: %v\n", err)
	}

	waEngine.Shutdown()
}
