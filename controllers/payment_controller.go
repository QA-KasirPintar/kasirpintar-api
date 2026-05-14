// controllers/payment_controller.go
package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"kasirpintar-api/config"
	"kasirpintar-api/models"

	"github.com/gin-gonic/gin"
)

func HandlePaymentNotification(c *gin.Context) {
	now := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("\n===== MIDTRANS WEBHOOK RECEIVED (%s) =====\n", now)

	// log headers
	fmt.Println("[HEADERS]")
	for k, v := range c.Request.Header {
		fmt.Printf("  %s: %v\n", k, v)
	}

	// read raw body
	rawBody, err := c.GetRawData()
	if err != nil {
		fmt.Println("[ERROR] GetRawData:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "gagal baca payload"})
		return
	}
	fmt.Println("[DEBUG] raw payload:", string(rawBody))

	// append to webhook.log for persistence
	f, ferr := os.OpenFile("webhook.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if ferr == nil {
		fmt.Fprintf(f, "==== %s ====\nHEADERS:\n", now)
		for k, v := range c.Request.Header {
			fmt.Fprintf(f, "%s: %v\n", k, v)
		}
		fmt.Fprintf(f, "BODY:\n%s\n\n", string(rawBody))
		f.Close()
	} else {
		fmt.Println("[WARN] cannot write webhook.log:", ferr)
	}

	// parse
	var payload map[string]interface{}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		fmt.Println("[ERROR] json unmarshal:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload tidak valid"})
		return
	}
	fmt.Printf("[DEBUG] parsed payload: %+v\n", payload)

	// extract order_id (unescape)
	rawOrderID, ok := payload["order_id"].(string)
	if !ok {
		fmt.Println("[ERROR] order_id not found in payload")
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_id tidak ditemukan"})
		return
	}
	fmt.Println("[DEBUG] raw order_id:", rawOrderID)
	orderID, err := url.QueryUnescape(strings.TrimSpace(rawOrderID))
	if err != nil {
		fmt.Println("[WARN] Unescape failed, using raw:", err)
		orderID = rawOrderID
	}
	fmt.Println("[DEBUG] normalized order_id:", orderID)

	// transaction_status
	statusStr, ok := payload["transaction_status"].(string)
	if !ok {
		fmt.Println("[ERROR] transaction_status not found in payload")
		c.JSON(http.StatusBadRequest, gin.H{"error": "transaction_status tidak ditemukan"})
		return
	}
	fmt.Println("[DEBUG] transaction_status:", statusStr)

	// lookup DB
	var trx models.Transaction
	res := config.DB.Where("invoice_number = ?", orderID).First(&trx)
	if res.Error != nil {
		fmt.Println("[ERROR] transaksi not found:", res.Error)
		c.JSON(http.StatusOK, gin.H{"message": "Notifikasi diterima (transaksi tidak ditemukan)."})
		return
	}
	fmt.Printf("[INFO] transaksi found id=%d invoice=%s status-db=%s total=%v\n", trx.ID, trx.InvoiceNumber, trx.Status, trx.TotalAmount)

	// proses settlement
	if statusStr == "settlement" {
		if trx.Status == "PENDING" {
			if err := config.DB.Model(&trx).Update("status", "PAID").Error; err != nil {
				fmt.Println("[ERROR] failed update status:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "gagal update db"})
				return
			}
			fmt.Printf("[INFO] transaksi id=%d updated to PAID\n", trx.ID)
		} else {
			fmt.Printf("[INFO] transaksi id=%d already %s; ignore\n", trx.ID, trx.Status)
		}
	} else {
		fmt.Printf("[INFO] transaction_status not settlement (%s); ignoring\n", statusStr)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notifikasi diterima"})
}
