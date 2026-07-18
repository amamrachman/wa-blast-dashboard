// Package whatsapp
package whatsapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

type WAEngine struct {
	Client *whatsmeow.Client
}

func NewWAEngine() *WAEngine {
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
	client := whatsmeow.NewClient(deviceStore, clientLog)

	return &WAEngine{Client: client}
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

func (wa *WAEngine) SendBlastMessage(number string, message string, imageData string) error {
	cleanNumber := strings.TrimSpace(number)
	cleanNumber = strings.ReplaceAll(cleanNumber, "+", "")
	cleanNumber = strings.ReplaceAll(cleanNumber, " ", "")
	cleanNumber = strings.ReplaceAll(cleanNumber, "-", "")

	if strings.HasPrefix(cleanNumber, "0") {
		cleanNumber = "62" + cleanNumber[1:]
	}

	targetJID := types.NewJID(cleanNumber, types.DefaultUserServer)
	var msg waProto.Message

	if imageData != "" {
		var uploadedMedia whatsmeow.UploadResponse
		var fileBytes []byte
		var mimeType string
		var err error

		commaIndex := strings.Index(imageData, ",")
		base64Data := imageData
		if commaIndex != -1 {
			base64Data = imageData[commaIndex+1:]
			meta := imageData[:commaIndex]
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
			return fmt.Errorf("gagal decode base64: %w", err)
		}

		uploadedMedia, err = wa.Client.Upload(context.Background(), fileBytes, whatsmeow.MediaImage)
		if err != nil {
			return fmt.Errorf("gagal upload ke server WA: %w", err)
		}

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

	_, err := wa.Client.SendMessage(context.Background(), targetJID, &msg)
	return err
}
