package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

type BlastPayload struct {
	Numbers   []string `json:"numbers"`
	Message   string   `json:"message"`
	ImageData string   `json:"image_data"`
}

var waClient *whatsmeow.Client

func connectToWhatsApp() {
	dbLog := waLog.Stdout("Database", "WARN", true)

	connStr := "file:whatsapp_session.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&cache=shared"

	container, err := sqlstore.New(context.Background(), "sqlite", connStr, dbLog)
	if err != nil {
		log.Fatalf("Gagal membuat database sesi: %v", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		log.Fatalf("Gagal mengambil data device: %v", err)
	}

	clientLog := waLog.Stdout("Client", "WARN", true)
	waClient = whatsmeow.NewClient(deviceStore, clientLog)

	if waClient.Store.ID == nil {
		qrChan, _ := waClient.GetQRChannel(context.Background())
		err = waClient.Connect()
		if err != nil {
			log.Fatalf("Gagal terhubung: %v", err)
		}

		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("\n==================================================")
				fmt.Println("👉 SILAHKAN SCAN QR CODE DI BAWAH INI DENGAN WA HP:")
				fmt.Println("==================================================")
				qrTerminal, err := qrcode.New(evt.Code, qrcode.Medium)
				if err == nil {
					fmt.Println(qrTerminal.ToSmallString(false))
				}
			}
		}
	} else {
		err = waClient.Connect()
		if err != nil {
			log.Fatalf("Gagal auto-connect: %v", err)
		}
		fmt.Println("✅ Sukses terhubung kembali ke WhatsApp secara otomatis!")
	}
}

func main() {
	go connectToWhatsApp()

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

		if waClient == nil || !waClient.IsConnected() {
			return c.Status(500).JSON(fiber.Map{"error": "Aplikasi belum terhubung ke WhatsApp"})
		}

		go func() {

			var uploadedMedia whatsmeow.UploadResponse
			var fileBytes []byte
			var mimeType string
			var err error

			hasImage := payload.ImageData != ""

			if hasImage {
				commaIndex := strings.Index(payload.ImageData, ",")
				base64Data := payload.ImageData
				if commaIndex != -1 {
					base64Data = payload.ImageData[commaIndex+1:]
					meta := payload.ImageData[:commaIndex]
					if strings.Contains(meta, "image/jpeg") || strings.Contains(meta, "image/jpg") {
						mimeType = "image/jpeg"
					} else if strings.Contains(meta, "image/png") {
						mimeType = "image/png"
					} else {
						mimeType = "image/jpeg"
					}
				}

				fileBytes, err = base64.StdEncoding.DecodeString(base64Data)
				if err != nil {
					fmt.Println("❌ Gagal decode data Base64 gambar:", err)
					return
				}

				uploadedMedia, err = waClient.Upload(context.Background(), fileBytes, whatsmeow.MediaImage)
				if err != nil {
					fmt.Println("❌ Gagal upload gambar ke server WhatsApp:", err)
					return
				}
			}

			fmt.Printf("\n🚀 Memulai Blast Media ke %d nomor...\n", len(payload.Numbers))

			for i, number := range payload.Numbers {
				cleanNumber := strings.TrimSpace(number)
				cleanNumber = strings.ReplaceAll(cleanNumber, "+", "")
				cleanNumber = strings.ReplaceAll(cleanNumber, " ", "")
				cleanNumber = strings.ReplaceAll(cleanNumber, "-", "")

				if strings.HasPrefix(cleanNumber, "0") {
					cleanNumber = "62" + cleanNumber[1:]
				}

				targetJID := types.NewJID(cleanNumber, types.DefaultUserServer)
				var msg waProto.Message

				if hasImage {
					msg.ImageMessage = &waProto.ImageMessage{
						Caption:       proto.String(payload.Message),
						Mimetype:      proto.String(mimeType),
						URL:           proto.String(uploadedMedia.URL),
						DirectPath:    proto.String(uploadedMedia.DirectPath),
						MediaKey:      uploadedMedia.MediaKey,
						FileEncSHA256: uploadedMedia.FileEncSHA256,
						FileSHA256:    uploadedMedia.FileSHA256,
						FileLength:    proto.Uint64(uint64(len(fileBytes))),
					}
				} else {
					msg.Conversation = proto.String(payload.Message)
				}

				_, err = waClient.SendMessage(context.Background(), targetJID, &msg)
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
			"message": "Proses blast teks/media berhasil dimulai di background.",
			"target":  len(payload.Numbers),
		})
	})

	log.Fatal(app.Listen(":3000"))
}
