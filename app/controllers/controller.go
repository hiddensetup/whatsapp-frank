package controllers

import (
	"context"
	"log"
	"os"
	"os/exec"

	"github.com/f100x/go-whatsapp-proxy/app/dto"
	"github.com/gofiber/fiber/v2"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type Controller struct {
	dbContainer *sqlstore.Container
	client      *whatsmeow.Client
}

var qrCode string

func NewController(db *sqlstore.Container) *Controller {
	cntrl := Controller{
		dbContainer: db,
	}

	clientLog := waLog.Stdout("Client", os.Getenv("LOG_LEVEL"), true)
	cntrl.client = whatsmeow.NewClient(cntrl.getDevice(), clientLog)
	cntrl.client.AddEventHandler(cntrl.eventHandler)

	return &cntrl
}

func (k *Controller) Login(c *fiber.Ctx) error {
	if k.client.Store.ID == nil {
		// No ID stored, new login
		if !k.client.IsConnected() {
			err := k.client.Connect()

			if err != nil {
				k.client.Log.Errorf("WhatsApp connection error: %s", err.Error())

				return c.SendStatus(500)
			}
		}

		//go func() { k.qrChan
		// client should be disconnected here
		k.client.Disconnect()
		// This must be called *before* Connect(). It will then listen to all the relevant events from the client.
		qrChan, err := k.client.GetQRChannel(context.Background())
		err = k.client.Connect()
		// connect should be after
		if err != nil {
			k.client.Log.Errorf("WhatsApp connection error: %s", err.Error())

			return c.SendStatus(500)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				//qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				qrCode = evt.Code

				if qrCode != "" {
					qrCodeImg, err := qrcode.Encode(qrCode, qrcode.Medium, 500)
					if err != nil {
						k.client.Log.Errorf("QR code generation error: %s", err.Error())

						c.SendStatus(500)
					}

					return c.Send(qrCodeImg)
				}
			} else {
				// login event that we do not catch
				//qrCode = ""
				break
			}
		}

	} else {
		// Already logged in, just connect
		if err := k.Autologin(); err != nil {
			k.client.Log.Errorf("WhatsApp connection error: %s", err.Error())

			return c.SendStatus(500)
		}

		return c.JSON(dto.Response{Status: true})
	}

	return c.JSON(dto.Response{Status: false})
}

func (k *Controller) Autologin() error {
	// autologin only when client is auth
	if k.client.Store.ID != nil && !k.client.IsConnected() {
		err := k.client.Connect()
		if err != nil {
			k.client.Log.Errorf("WhatsApp connection error: %s", err.Error())
			return err
		}
	}

	return nil
}
func (k *Controller) Logout(c *fiber.Ctx) error {
	// Remove the whatsappstore.db file if it exists
	if _, err := os.Stat("whatsappstore.db"); err == nil {
		if err := os.Remove("whatsappstore.db"); err != nil {
			// Handle the error more gracefully, log it, and continue
			log.Printf("Error removing whatsappstore.db: %s", err)
		}
	}

	// Log out the user
	if k.client != nil {
		if err := k.client.Logout(); err != nil {
			// Handle the error more gracefully, return an error response
			return c.JSON(dto.Response{Status: false})
		}
	}

	// Run the run.sh script
	cmd := exec.Command("./run.sh")
	if err := cmd.Start(); err != nil {
		// Handle the error more gracefully, return an error response
		return c.JSON(dto.Response{Status: false})
	}

	return c.JSON(dto.Response{Status: true})
}

func (k *Controller) getDevice() *store.Device {
	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := k.dbContainer.GetFirstDevice()

	if err != nil {
		k.client.Log.Errorf("Device getting error: %s", err.Error())

		return nil
	}

	return deviceStore
}

func (k *Controller) GetClient() *whatsmeow.Client {
	return k.client
}
