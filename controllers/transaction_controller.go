// LOKASI: controllers/transaction_controller.go (GANTI TOTAL)

package controllers

import (
	"bytes"
	"fmt"
	"kasirpintar-api/config"
	"kasirpintar-api/models"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
)

type TransactionItemInput struct {
	MenuID   uint `json:"menu_id" binding:"required"`
	Quantity int  `json:"quantity" binding:"required,gt=0"`
}

type TransactionInput struct {
	Items         []TransactionItemInput `json:"items" binding:"required,min=1"`
	PaymentMethod string                 `json:"payment_method"`
	CashTendered  float64                `json:"cash_tandered"`
	Change        float64                `json:"change"`
	Discount      float64                `json:"discount"`
	CustomerID    *uint                  `json:"customer_id"`
	VoucherID     *uint                  `json:"voucher_id"`
}

func CreateTransaction(c *gin.Context) {
	var input TransactionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}

	userI, _ := c.Get("user")
	currentUser := userI.(models.User)
	if currentUser.OutletID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak. Hanya staf outlet yang bisa membuat transaksi."})
		return
	}

	initialStatus := "PENDING"
	if strings.ToLower(input.PaymentMethod) == "tunai" || strings.ToLower(input.PaymentMethod) == "cash" {
		initialStatus = "PAID"
	}

	var subtotal float64 = 0
	var transactionDetails []models.TransactionDetail
	var newTransaction models.Transaction

	var outlet models.Outlet
	if err := config.DB.Preload("TaxRate").First(&outlet, *currentUser.OutletID).Error; err != nil {
		outlet = models.Outlet{}
	}

	var appliedTax *models.TaxRate
	if outlet.TaxRate != nil {
		appliedTax = outlet.TaxRate
	} else if outlet.RegionName != nil && *outlet.RegionName != "" {
		var tr models.TaxRate
		if err := config.DB.Where("region = ? AND is_active = ?", *outlet.RegionName, true).
			Order("id desc").First(&tr).Error; err == nil {
			appliedTax = &tr
		}
	}

	err := config.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range input.Items {
			var menu models.Menu
			if err := tx.First(&menu, item.MenuID).Error; err != nil {
				return fmt.Errorf("menu dengan ID %d tidak ditemukan", item.MenuID)
			}
			// Pastikan menu milik outlet user
			if menu.OutletID != *currentUser.OutletID {
				return fmt.Errorf("menu %s bukan milik outlet ini", menu.Name)
			}
			if menu.Stock < item.Quantity {
				return fmt.Errorf("stok %s tidak mencukupi (%d tersedia)", menu.Name, menu.Stock)
			}

			newStock := menu.Stock - item.Quantity
			if err := tx.Model(&menu).Update("stock", newStock).Error; err != nil {
				return fmt.Errorf("gagal mengurangi stok %s: %w", menu.Name, err)
			}

			itemSubtotal := menu.Price * float64(item.Quantity)
			subtotal += itemSubtotal
			transactionDetails = append(transactionDetails, models.TransactionDetail{
				MenuID:   item.MenuID,
				Quantity: item.Quantity,
				Price:    menu.Price,
			})
		}

		taxable := subtotal - input.Discount
		if taxable < 0 {
			taxable = 0
		}

		var taxPercent float64 = 0
		var taxRateID *uint = nil
		if appliedTax != nil {
			taxPercent = appliedTax.RatePercent
			taxRateID = &appliedTax.ID
		}

		taxAmount := taxable * (taxPercent / 100.0)
		taxAmount = math.Round(taxAmount*100) / 100

		finalAmount := taxable + taxAmount
		if finalAmount < 0 {
			finalAmount = 0
		}

		paymentMethod := input.PaymentMethod
		if paymentMethod == "" {
			paymentMethod = "Tunai"
		}

		invoiceNumber := fmt.Sprintf("INV/%d/%d", time.Now().UnixNano(), *currentUser.OutletID)

		var taxPercentPtr *float64
		var taxAmountPtr *float64
		if taxRateID != nil {
			taxPercentPtr = new(float64)
			*taxPercentPtr = taxPercent
			taxAmountPtr = new(float64)
			*taxAmountPtr = taxAmount
		}

		transaction := models.Transaction{
			InvoiceNumber: invoiceNumber,
			Subtotal:      subtotal,
			Discount:      input.Discount,
			TotalAmount:   finalAmount,
			UserID:        currentUser.ID,
			OutletID:      *currentUser.OutletID,
			PaymentMethod: paymentMethod,
			CashTendered:  input.CashTendered,
			Change:        input.Change,
			CustomerID:    input.CustomerID,
			Details:       transactionDetails,
			Status:        initialStatus,
			TaxRateID:     taxRateID,
			TaxPercent:    taxPercentPtr,
			TaxAmount:     taxAmountPtr,
		}

		if err := tx.Create(&transaction).Error; err != nil {
			return fmt.Errorf("gagal menyimpan transaksi: %w", err)
		}
		newTransaction = transaction

		fmt.Println("[CREATE TRANSACTION] invoice:", newTransaction.InvoiceNumber, "id:", newTransaction.ID, "total:", newTransaction.TotalAmount, "payment_method:", newTransaction.PaymentMethod)

		if input.VoucherID != nil {
			var voucher models.Voucher
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&voucher, *input.VoucherID).Error; err != nil {
				return fmt.Errorf("voucher dengan ID %d tidak ditemukan", *input.VoucherID)
			}
			if voucher.IsUsed {
				return fmt.Errorf("voucher %s sudah pernah digunakan", voucher.Code)
			}
			var promotion models.Promotion
			if err := tx.First(&promotion, voucher.PromotionID).Error; err != nil {
				return fmt.Errorf("promosi untuk voucher %s tidak ditemukan", voucher.Code)
			}
			if promotion.OutletID != *currentUser.OutletID {
				return fmt.Errorf("voucher %s tidak berlaku di outlet ini", voucher.Code)
			}
			updateData := map[string]interface{}{
				"is_used":                true,
				"used_at":                time.Now(),
				"used_by_transaction_id": transaction.ID,
			}
			if err := tx.Model(&voucher).Updates(updateData).Error; err != nil {
				return fmt.Errorf("gagal mengupdate status voucher %s: %w", voucher.Code, err)
			}
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var qrString string = ""
	var midtransResponse *coreapi.ChargeResponse

	if strings.ToUpper(newTransaction.PaymentMethod) == "QRIS" {
		chargeReq := &coreapi.ChargeReq{
			PaymentType: coreapi.PaymentTypeQris,
			TransactionDetails: midtrans.TransactionDetails{
				OrderID:  newTransaction.InvoiceNumber,
				GrossAmt: int64(math.Round(newTransaction.TotalAmount)),
			},
			CustomExpiry: &coreapi.CustomExpiry{
				ExpiryDuration: 15,
				Unit:           "minute",
			},
		}

		resp, err := MidtransCoreAPI.ChargeTransaction(chargeReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaksi tersimpan, tapi gagal membuat QR Code: " + err.Error()})
			return
		}
		qrString = resp.QRString
		midtransResponse = resp
	}

	var createdTransaction models.Transaction
	config.DB.Preload("User").
		Preload("Customer").
		Preload("Outlet").
		Preload("Details.Menu").
		First(&createdTransaction, newTransaction.ID)

	var taxPercent float64 = 0
	var taxAmount float64 = 0
	if createdTransaction.TaxPercent != nil {
		taxPercent = *createdTransaction.TaxPercent
	}
	if createdTransaction.TaxAmount != nil {
		taxAmount = *createdTransaction.TaxAmount
	}

	c.JSON(http.StatusCreated, gin.H{
		"data": gin.H{
			"ID":            createdTransaction.ID,
			"InvoiceNumber": createdTransaction.InvoiceNumber,
			"Subtotal":      createdTransaction.Subtotal,
			"Discount":      createdTransaction.Discount,
			"TaxPercent":    taxPercent,
			"TaxAmount":     taxAmount,
			"TotalAmount":   createdTransaction.TotalAmount,
			"PaymentMethod": createdTransaction.PaymentMethod,
			"CashTendered":  createdTransaction.CashTendered,
			"Change":        createdTransaction.Change,
			"Status":        createdTransaction.Status,
			"User":          createdTransaction.User,
			"Customer":      createdTransaction.Customer,
			"Details":       createdTransaction.Details,
			"CreatedAt":     createdTransaction.CreatedAt,
		},
		"qr_string":         qrString,
		"midtrans_response": midtransResponse,
	})
}

func buildFilteredTransactionsQuery(c *gin.Context) (*gorm.DB, error) {
	user, _ := c.Get("user")
	currentUser := user.(models.User)
	var outletID uint
	outletIDQuery := c.Query("outlet_id")
	if currentUser.Role == "owner" {
		if outletIDQuery == "" {
			return nil, fmt.Errorf("Owner harus memilih outlet (?outlet_id=...)")
		}
		parsedID, err := strconv.ParseUint(outletIDQuery, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Format outlet_id tidak valid")
		}
		outletID = uint(parsedID)
	} else {
		if currentUser.OutletID == nil {
			return nil, fmt.Errorf("Akses ditolak. Akun Anda tidak terikat ke outlet manapun.")
		}
		outletID = *currentUser.OutletID
	}
	baseQuery := config.DB.Model(&models.Transaction{}).
		Preload("User").
		Preload("Customer").
		Preload("Details.Menu").
		Where("transactions.outlet_id = ?", outletID)

	searchQuery := c.Query("search")
	if searchQuery != "" {
		searchPattern := "%" + strings.ToLower(searchQuery) + "%"
		baseQuery = baseQuery.Joins("LEFT JOIN customers ON customers.id = transactions.customer_id").
			Where("LOWER(transactions.invoice_number) LIKE ? OR LOWER(customers.name) LIKE ?", searchPattern, searchPattern)
	}

	startDateQuery := c.Query("start_date")
	endDateQuery := c.Query("end_date")
	var startTime time.Time
	var endTime time.Time
	var err error
	if startDateQuery != "" {
		startTime, err = time.Parse("2006-01-02", startDateQuery)
		if err != nil {
			return nil, fmt.Errorf("Format start_date tidak valid. Gunakan YYYY-MM-DD.")
		}
		startTime = time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, startTime.Location())
	}
	if endDateQuery != "" {
		endTime, err = time.Parse("2006-01-02", endDateQuery)
		if err != nil {
			return nil, fmt.Errorf("Format end_date tidak valid. Gunakan YYYY-MM-DD.")
		}
	}
	if !startTime.IsZero() && endTime.IsZero() {
		endTime = time.Now()
	}
	if !startTime.IsZero() && !endTime.IsZero() {
		endTime = time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 23, 59, 59, 999999999, endTime.Location())
		baseQuery = baseQuery.Where("transactions.created_at BETWEEN ? AND ?", startTime, endTime)
	} else if !startTime.IsZero() {
		baseQuery = baseQuery.Where("transactions.created_at >= ?", startTime)
	} else if !endTime.IsZero() {
		endTime = time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 23, 59, 59, 999999999, endTime.Location())
		baseQuery = baseQuery.Where("transactions.created_at <= ?", endTime)
	}

	return baseQuery, nil
}

func GetTransactions(c *gin.Context) {
	baseQuery, err := buildFilteredTransactionsQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var totalItems int64
	countQuery := *baseQuery
	if err := countQuery.Count(&totalItems).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghitung total transaksi."})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit
	totalPages := int(math.Ceil(float64(totalItems) / float64(limit)))
	startDateQuery := c.Query("start_date")
	endDateQuery := c.Query("end_date")
	sortDirection := "desc"
	if startDateQuery != "" || endDateQuery != "" {
		sortDirection = "asc"
	}
	var transactions []models.Transaction
	err = baseQuery.Order("created_at " + sortDirection).
		Offset(offset).
		Limit(limit).
		Find(&transactions).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil riwayat transaksi."})
		return
	}
	if transactions == nil {
		transactions = make([]models.Transaction, 0)
	}
	c.JSON(http.StatusOK, gin.H{
		"data": transactions,
		"pagination": gin.H{
			"total_items":  totalItems,
			"total_pages":  totalPages,
			"current_page": page,
			"per_page":     limit,
		},
	})
}

func ExportTransactions(c *gin.Context) {
	user, _ := c.Get("user")
	currentUser := user.(models.User)
	var outletID uint
	outletIDQuery := c.Query("outlet_id")
	if currentUser.Role == "owner" {
		if outletIDQuery == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Owner harus memilih outlet (?outlet_id=...)"})
			return
		}
		parsedID, err := strconv.ParseUint(outletIDQuery, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format outlet_id tidak valid"})
			return
		}
		outletID = uint(parsedID)
	} else {
		if currentUser.OutletID == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak. Akun Anda tidak terikat ke outlet manapun."})
			return
		}
		outletID = *currentUser.OutletID
	}
	var outlet models.Outlet
	if err := config.DB.First(&outlet, outletID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menemukan detail outlet."})
		return
	}
	outletName := outlet.Name
	reportCreatorName := currentUser.Name
	baseQuery := config.DB.Model(&models.Transaction{}).
		Preload("User").
		Preload("Customer").
		Where("transactions.outlet_id = ?", outletID)
	searchQuery := c.Query("search")
	if searchQuery != "" {
		searchPattern := "%" + strings.ToLower(searchQuery) + "%"
		baseQuery = baseQuery.Joins("LEFT JOIN customers ON customers.id = transactions.customer_id").
			Where("LOWER(transactions.invoice_number) LIKE ? OR LOWER(customers.name) LIKE ?", searchPattern, searchPattern)
	}
	startDateQuery := c.Query("start_date")
	endDateQuery := c.Query("end_date")
	var startTime time.Time
	var endTime time.Time
	var err error
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.UTC
	}
	if startDateQuery != "" {
		startTime, err = time.Parse("2006-01-02", startDateQuery)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format start_date tidak valid. Gunakan YYYY-MM-DD."})
			return
		}
		startTime = time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, startTime.Location())
	}
	if endDateQuery != "" {
		endTime, err = time.Parse("2006-01-02", endDateQuery)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format end_date tidak valid. Gunakan YYYY-MM-DD."})
			return
		}
	}
	if !startTime.IsZero() && endTime.IsZero() {
		endTime = time.Now().In(loc)
	}
	if !startTime.IsZero() && !endTime.IsZero() {
		endTime = time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 23, 59, 59, 999999999, endTime.Location())
		baseQuery = baseQuery.Where("transactions.created_at BETWEEN ? AND ?", startTime, endTime)
	} else if !startTime.IsZero() {
		baseQuery = baseQuery.Where("transactions.created_at >= ?", startTime)
	} else if !endTime.IsZero() {
		endTime = time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 23, 59, 59, 999999999, endTime.Location())
		baseQuery = baseQuery.Where("transactions.created_at <= ?", endTime)
	}
	sortDirection := "desc"
	if startDateQuery != "" || endDateQuery != "" {
		sortDirection = "asc"
	}
	var transactions []models.Transaction
	err = baseQuery.Order("created_at " + sortDirection).Find(&transactions).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data untuk ekspor."})
		return
	}
	exportType := c.DefaultQuery("type", "excel")
	if exportType == "excel" {
		f := excelize.NewFile()
		sheetName := "Riwayat Transaksi"
		f.SetSheetName("Sheet1", sheetName)
		var dateRangeString string
		var filenameDatePart string
		var startDisplay, endDisplay string
		if startDateQuery != "" {
			t, _ := time.Parse("2006-01-02", startDateQuery)
			startDisplay = t.Format("02-01-2006")
		}
		if endDateQuery != "" {
			t, _ := time.Parse("2006-01-02", endDateQuery)
			endDisplay = t.Format("02-01-2006")
		}
		if startDateQuery != "" && endDateQuery != "" {
			dateRangeString = fmt.Sprintf("Data Transaksi: dari tgl %s ke tgl %s", startDisplay, endDisplay)
			filenameDatePart = fmt.Sprintf("%s_sd_%s", startDateQuery, endDateQuery)
		} else if startDateQuery != "" {
			endDisplay = time.Now().In(loc).Format("02-01-2006")
			dateRangeString = fmt.Sprintf("Data Transaksi: dari tgl %s ke tgl %s", startDisplay, endDisplay)
			filenameDatePart = fmt.Sprintf("%s_sd_%s", startDateQuery, time.Now().Format("2006-01-02"))
		} else if endDateQuery != "" {
			dateRangeString = fmt.Sprintf("Data Transaksi: sampai tgl %s", endDisplay)
			filenameDatePart = fmt.Sprintf("sampai_%s", endDateQuery)
		} else {
			dateRangeString = "Data Transaksi: Semua Data"
			filenameDatePart = "SemuaData"
		}
		generationTime := time.Now().In(loc).Format("02 Jan 2006, 15:04:05 WIB")
		titleRow1 := "Laporan Riwayat Transaksi"
		titleRow2 := fmt.Sprintf("Outlet: %s", outletName)
		titleRow3 := dateRangeString
		titleRow4 := fmt.Sprintf("Dibuat oleh %s pada %s", reportCreatorName, generationTime)
		f.MergeCell(sheetName, "A1", "I1")
		f.MergeCell(sheetName, "A2", "I2")
		f.MergeCell(sheetName, "A3", "I3")
		f.MergeCell(sheetName, "A4", "I4")
		titleStyle, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Size: 14},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		subTitleStyle, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		f.SetCellStyle(sheetName, "A1", "A1", titleStyle)
		f.SetCellValue(sheetName, "A1", titleRow1)
		f.SetCellStyle(sheetName, "A2", "A2", subTitleStyle)
		f.SetCellValue(sheetName, "A2", titleRow2)
		f.SetCellStyle(sheetName, "A3", "A3", subTitleStyle)
		f.SetCellValue(sheetName, "A3", titleRow3)
		f.SetCellStyle(sheetName, "A4", "A4", subTitleStyle)
		f.SetCellValue(sheetName, "A4", titleRow4)
		f.SetRowHeight(sheetName, 1, 25)
		f.SetRowHeight(sheetName, 2, 20)
		f.SetRowHeight(sheetName, 3, 20)
		f.SetRowHeight(sheetName, 4, 20)
		headers := []string{"Invoice", "Tanggal", "Waktu", "Kasir", "Pelanggan", "Subtotal", "Diskon", "Total", "Metode Bayar"}
		headerStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 5)
			f.SetCellValue(sheetName, cell, h)
			f.SetCellStyle(sheetName, cell, cell, headerStyle)
		}
		for i, trx := range transactions {
			row := i + 6
			customerName := "Pelanggan Umum"
			if trx.Customer.ID != 0 {
				customerName = trx.Customer.Name
			}
			kasirName := "N/A"
			if trx.User.ID != 0 {
				kasirName = trx.User.Name
			}
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), trx.InvoiceNumber)
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), trx.CreatedAt.Format("2006-01-02"))
			f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), trx.CreatedAt.Format("15:04:05"))
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), kasirName)
			f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), customerName)
			f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), trx.Subtotal)
			f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), trx.Discount)
			f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), trx.TotalAmount)
			f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), trx.PaymentMethod)
		}
		var b bytes.Buffer
		if err := f.Write(&b); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat file Excel."})
			return
		}
		safeOutletName := strings.ReplaceAll(outletName, " ", "_")
		filename := fmt.Sprintf("Laporan_Transaksi_%s_%s.xlsx", safeOutletName, filenameDatePart)
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Disposition", "attachment; filename="+filename)
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", b.Bytes())
	} else if exportType == "pdf" {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Ekspor PDF belum didukung."})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tipe ekspor tidak valid."})
	}
}

func GetTransactionStatusByInvoice(c *gin.Context) {
	invoiceParam := c.Param("invoice")
	invoiceNumber := strings.TrimPrefix(invoiceParam, "/")
	user, _ := c.Get("user")
	currentUser := user.(models.User)

	var transaction models.Transaction
	query := config.DB.Where("invoice_number = ?", invoiceNumber).First(&transaction)

	if query.Error != nil {
		if query.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Transaksi tidak ditemukan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data transaksi"})
		return
	}

	if currentUser.Role != "owner" && currentUser.Role != "admin" {
		if transaction.OutletID != *currentUser.OutletID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak untuk invoice ini"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         transaction.Status,
		"id":             transaction.ID,
		"invoice_number": transaction.InvoiceNumber,
	})
}

func PreviewTransaction(c *gin.Context) {
	var input TransactionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userI, _ := c.Get("user")
	currentUser := userI.(models.User)

	if currentUser.OutletID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Outlet tidak ditemukan"})
		return
	}

	var subtotal float64 = 0

	for _, item := range input.Items {
		var menu models.Menu
		if err := config.DB.First(&menu, item.MenuID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Menu tidak ditemukan"})
			return
		}
		subtotal += menu.Price * float64(item.Quantity)
	}

	var outlet models.Outlet
	config.DB.Preload("TaxRate").First(&outlet, *currentUser.OutletID)

	var taxPercent float64 = 0
	if outlet.TaxRate != nil {
		taxPercent = outlet.TaxRate.RatePercent
	}

	taxable := subtotal - input.Discount
	if taxable < 0 {
		taxable = 0
	}

	taxAmount := taxable * (taxPercent / 100)
	taxAmount = math.Round(taxAmount*100) / 100

	total := taxable + taxAmount

	c.JSON(http.StatusOK, gin.H{
		"subtotal":    subtotal,
		"tax_percent": taxPercent,
		"tax_amount":  taxAmount,
		"total":       total,
	})
}
