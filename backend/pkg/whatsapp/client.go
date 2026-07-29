// Package whatsapp
package whatsapp

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

var onlyDigitsPlus = regexp.MustCompile(`^\d+$`)

type WAEngine struct {
	Client *whatsmeow.Client
	DB     *sql.DB

	isSending int32
}

func NewWAEngine() *WAEngine {
	dbLog := waLog.Stdout("Database", "ERROR", true)
	connStr := "file:whatsapp_session.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&cache=shared"

	container, err := sqlstore.New(context.Background(), "sqlite", connStr, dbLog)
	if err != nil {
		log.Fatalf("Gagal membuat database sesi: %v", err)
	}

	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		log.Fatalf("Gagal membuka koneksi database aplikasi: %v", err)
	}

	initScheduleTable(db)

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		log.Fatalf("Gagal mengambil data device: %v", err)
	}

	clientLog := waLog.Stdout("Client", "ERROR", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	engine := &WAEngine{
		Client: client,
		DB:     db,
	}

	engine.recoverStuckJobs()
	engine.warnPendingSchedulesOnManualMode()

	return engine
}

func initScheduleTable(db *sql.DB) {
	query := `
	CREATE TABLE IF NOT EXISTS scheduled_blasts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		numbers TEXT NOT NULL,
		message TEXT NOT NULL,
		image_data TEXT,
		scheduled_at DATETIME NOT NULL,
		status VARCHAR(20) DEFAULT 'PENDING',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := db.Exec(query)
	if err != nil {
		log.Printf("⚠️ Gagal membuat tabel scheduled_blasts: %v\n", err)
	}
}

func (wa *WAEngine) recoverStuckJobs() {
	res, err := wa.DB.Exec(`UPDATE scheduled_blasts SET status = 'PENDING' WHERE status = 'PROCESSING'`)
	if err != nil {
		log.Printf("⚠️ Gagal recovery job yang macet: %v\n", err)
		return
	}
	if n, _ := res.RowsAffected(); n > 0 {
		log.Printf("🔁 %d jadwal sebelumnya ditemukan macet di status PROCESSING, dikembalikan ke PENDING.\n", n)
	}
}

func (wa *WAEngine) warnPendingSchedulesOnManualMode() {
	rows, err := wa.DB.Query(`SELECT id, scheduled_at FROM scheduled_blasts WHERE status = 'PENDING' ORDER BY scheduled_at ASC`)
	if err != nil {
		return
	}
	defer rows.Close()

	count := 0
	var earliest time.Time
	for rows.Next() {
		var id int
		var scheduledAt time.Time
		if err := rows.Scan(&id, &scheduledAt); err != nil {
			continue
		}
		if count == 0 || scheduledAt.Before(earliest) {
			earliest = scheduledAt
		}
		count++
	}

	if count > 0 {
		fmt.Println("\n==================================================")
		fmt.Printf("⚠️  PERHATIAN: Ada %d jadwal PENDING di database.\n", count)
		fmt.Printf("    Jadwal terdekat: %s\n", earliest.Format("2006-01-02 15:04:05"))
		fmt.Println("    Jadwal HANYA akan terkirim jika proses ini (go run)")
		fmt.Println("    tetap berjalan sampai waktu tersebut. Jangan tutup terminal!")
		fmt.Println("==================================================")
	}
}

func (wa *WAEngine) Connect() {
	if wa.Client.Store.ID == nil {
		qrChan, _ := wa.Client.GetQRChannel(context.Background())
		err := wa.Client.Connect()
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
		err := wa.Client.Connect()
		if err != nil {
			log.Fatalf("Gagal auto-connect: %v", err)
		}
		fmt.Println("✅ Sukses terhubung kembali ke WhatsApp secara otomatis!")
	}
}

func (wa *WAEngine) Shutdown() {
	if atomic.LoadInt32(&wa.isSending) == 1 {
		fmt.Println("⏳ Menunggu proses blast yang sedang berjalan selesai mengirim nomor saat ini...")
		for atomic.LoadInt32(&wa.isSending) == 1 {
			time.Sleep(200 * time.Millisecond)
		}
	}
	if wa.Client != nil {
		wa.Client.Disconnect()
	}
	if wa.DB != nil {
		wa.DB.Close()
	}
	fmt.Println("👋 WAEngine berhasil dimatikan dengan aman.")
}

func detectMimeType(imageData string) string {
	switch {
	case strings.Contains(imageData, "image/png"):
		return "image/png"
	case strings.Contains(imageData, "image/webp"):
		return "image/webp"
	case strings.Contains(imageData, "image/jpeg"), strings.Contains(imageData, "image/jpg"):
		return "image/jpeg"
	default:
		return "image/jpeg"
	}
}

func (wa *WAEngine) PrepareImage(imageData string) (whatsmeow.UploadResponse, []byte, string, error) {
	var uploadedMedia whatsmeow.UploadResponse

	if strings.TrimSpace(imageData) == "" {
		return uploadedMedia, nil, "", fmt.Errorf("data gambar kosong")
	}

	mimeType := detectMimeType(imageData)

	base64Data := imageData
	if commaIndex := strings.Index(imageData, ","); commaIndex != -1 {
		base64Data = imageData[commaIndex+1:]
	}

	fileBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return uploadedMedia, nil, "", fmt.Errorf("gagal decode base64: %w", err)
	}
	if len(fileBytes) == 0 {
		return uploadedMedia, nil, "", fmt.Errorf("gambar kosong setelah decode")
	}

	uploadedMedia, err = wa.Client.Upload(context.Background(), fileBytes, whatsmeow.MediaImage)
	if err != nil {
		return uploadedMedia, nil, "", fmt.Errorf("gagal upload ke server WA: %w", err)
	}

	return uploadedMedia, fileBytes, mimeType, nil
}

func NormalizeNumber(number string) (string, error) {
	clean := strings.TrimSpace(number)
	clean = strings.ReplaceAll(clean, "+", "")
	clean = strings.ReplaceAll(clean, " ", "")
	clean = strings.ReplaceAll(clean, "-", "")
	clean = strings.ReplaceAll(clean, "(", "")
	clean = strings.ReplaceAll(clean, ")", "")

	if clean == "" {
		return "", fmt.Errorf("nomor kosong")
	}

	if strings.HasPrefix(clean, "0") {
		clean = "62" + clean[1:]
	}

	if !onlyDigitsPlus.MatchString(clean) {
		return "", fmt.Errorf("nomor '%s' mengandung karakter tidak valid", number)
	}

	if len(clean) < 10 || len(clean) > 16 {
		return "", fmt.Errorf("nomor '%s' memiliki panjang yang tidak wajar", number)
	}

	return clean, nil
}

func (wa *WAEngine) SendPreparedMessage(number string, message string, uploadedMedia *whatsmeow.UploadResponse, fileBytes []byte, mimeType string) error {
	cleanNumber, err := NormalizeNumber(number)
	if err != nil {
		return err
	}

	if wa.Client == nil || !wa.Client.IsConnected() {
		return fmt.Errorf("koneksi WhatsApp terputus")
	}

	targetJID := types.NewJID(cleanNumber, types.DefaultUserServer)
	var msg waProto.Message

	if uploadedMedia != nil {
		msg.ImageMessage = &waProto.ImageMessage{
			Caption:       proto.String(message),
			Mimetype:      proto.String(mimeType),
			URL:           proto.String(uploadedMedia.URL),
			DirectPath:    proto.String(uploadedMedia.DirectPath),
			MediaKey:      uploadedMedia.MediaKey,
			FileEncSHA256: uploadedMedia.FileEncSHA256,
			FileSHA256:    uploadedMedia.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(fileBytes))),
		}
	} else {
		msg.Conversation = proto.String(message)
	}

	_, err = wa.Client.SendMessage(context.Background(), targetJID, &msg)
	return err
}

func (wa *WAEngine) ScheduleBlastMessage(numbers []string, message string, imageData string, scheduledAt time.Time) error {
	numbersJoined := strings.Join(numbers, ",")
	query := `INSERT INTO scheduled_blasts (numbers, message, image_data, scheduled_at, status) VALUES (?, ?, ?, ?, 'PENDING')`
	_, err := wa.DB.Exec(query, numbersJoined, message, imageData, scheduledAt)
	return err
}

func (wa *WAEngine) MarkSending() {
	atomic.StoreInt32(&wa.isSending, 1)
}

func (wa *WAEngine) MarkDone() {
	atomic.StoreInt32(&wa.isSending, 0)
}
