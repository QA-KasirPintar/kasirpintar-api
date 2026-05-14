// LOKASI: kasirpintar-api/controllers/dashboard_controller.go
package controllers

import (
	"bytes"
	"encoding/json"
	"kasirpintar-api/config"
	"kasirpintar-api/models"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type TopProductResult struct {
	Name          string `json:"name"`
	TotalQuantity int    `json:"total_quantity"`
}

type DailySaleResult struct {
	Date  string  `json:"date"`
	Total float64 `json:"total"`
}

type PaymentMethodResult struct {
	PaymentMethod string `json:"payment_method"`
	Count         int    `json:"count"`
}

type BasketAnalysisResult struct {
	Product1  string `json:"product_1"`
	Product2  string `json:"product_2"`
	Frequency int    `json:"frequency"`
}

type BusyHourResult struct {
	DayOfWeek int `json:"day_of_week"`
	Hour      int `json:"hour"`
	Count     int `json:"count"`
}

type CustomerRFM struct {
	CustomerID   uint      `json:"customer_id"`
	Name         string    `json:"name"`
	PhoneNumber  string    `json:"phone_number"`
	LastPurchase time.Time `json:"last_purchase"` // Recency
	Frequency    int64     `json:"frequency"`     // Frequency
	Monetary     float64   `json:"monetary"`      // Monetary
	Segment      string    `json:"segment"`
}

// --- FUNGSI GetDashboardSummary TELAH DIPERBARUI TOTAL ---
func GetDashboardSummary(c *gin.Context) {
	user, _ := c.Get("user")
	currentUser := user.(models.User)

	// Ambil parameter dari query URL
	layout := "2006-01-02"
	startDateStr := c.DefaultQuery("start_date", time.Now().AddDate(0, 0, -6).Format(layout))
	endDateStr := c.DefaultQuery("end_date", time.Now().Format(layout))
	outletIDQuery := c.Query("outlet_id") // Parameter baru untuk filter Owner

	startDate, err1 := time.Parse(layout, startDateStr)
	endDate, err2 := time.Parse(layout, endDateStr)

	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal tidak valid, gunakan YYYY-MM-DD"})
		return
	}

	// Atur waktu agar mencakup satu hari penuh
	loc := time.Local
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, loc)
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 0, loc)

	// --- LOGIKA FILTER DINAMIS BERDASARKAN PERAN ---
	db := config.DB.Where("created_at BETWEEN ? AND ?", startDate, endDate)

	if currentUser.Role == "owner" {
		// Jika owner memilih outlet tertentu dari dropdown, filter berdasarkan itu
		if outletIDQuery != "" {
			db = db.Where("outlet_id = ?", outletIDQuery)
		}
		// Jika tidak, biarkan query tanpa filter outlet (ambil dari semua)
	} else {
		// Untuk peran lain (branch_manager, dll), selalu filter berdasarkan outlet mereka.
		// Ini memastikan fungsionalitas lama tidak berubah.
		db = db.Where("outlet_id = ?", currentUser.OutletID)
	}

	// --- KALKULASI METRIK DASAR ---
	var totalRevenue float64
	db.Model(&models.Transaction{}).Select("COALESCE(SUM(total_amount), 0)").Row().Scan(&totalRevenue)

	var totalTransactions int64
	db.Model(&models.Transaction{}).Count(&totalTransactions)

	var totalDiscount float64
	db.Model(&models.Transaction{}).Select("COALESCE(SUM(discount), 0)").Row().Scan(&totalDiscount)

	var avgPerTransaction float64
	if totalTransactions > 0 {
		avgPerTransaction = totalRevenue / float64(totalTransactions)
	}

	// --- KALKULASI METRIK TAMBAHAN DENGAN FILTER YANG SAMA ---
	// Query untuk 'newCustomers' juga menggunakan filter dinamis
	customerQuery := config.DB.Model(&models.Customer{}).Where("created_at BETWEEN ? AND ?", startDate, endDate)
	if currentUser.Role == "owner" {
		if outletIDQuery != "" {
			customerQuery = customerQuery.Where("outlet_id = ?", outletIDQuery)
		}
	} else {
		customerQuery = customerQuery.Where("outlet_id = ?", currentUser.OutletID)
	}
	var newCustomers int64
	customerQuery.Count(&newCustomers)

	// Query untuk 'topProducts' juga menggunakan filter dinamis
	topProductsQuery := config.DB.Table("transaction_details").
		Select("menus.name, SUM(transaction_details.quantity) as total_quantity").
		Joins("join menus on menus.id = transaction_details.menu_id").
		Joins("join transactions on transactions.id = transaction_details.transaction_id").
		// Where("transactions.created_at BETWEEN ?", startDate, endDate)
		Where("transactions.created_at BETWEEN ? AND ?", startDate, endDate)

	if currentUser.Role == "owner" {
		if outletIDQuery != "" {
			topProductsQuery = topProductsQuery.Where("transactions.outlet_id = ?", outletIDQuery)
		}
	} else {
		topProductsQuery = topProductsQuery.Where("transactions.outlet_id = ?", currentUser.OutletID)
	}
	var topProducts []TopProductResult
	topProductsQuery.Group("menus.name").Order("total_quantity DESC").Limit(5).Scan(&topProducts)

	var paymentMethods []PaymentMethodResult
	db.Model(&models.Transaction{}).Select("payment_method, COUNT(*) as count").Group("payment_method").Scan(&paymentMethods)

	var salesByDay []DailySaleResult
	db.Model(&models.Transaction{}).Select("DATE(created_at) as date, SUM(total_amount) as total").Group("DATE(created_at)").Order("date ASC").Scan(&salesByDay)

	c.JSON(http.StatusOK, gin.H{
		"total_revenue":              totalRevenue,
		"total_transactions":         totalTransactions,
		"new_customers":              newCustomers,
		"top_products":               topProducts,
		"sales_by_day":               salesByDay,
		"total_discount":             totalDiscount,
		"avg_per_transaction":        avgPerTransaction,
		"payment_method_composition": paymentMethods,
	})
}

// --- FUNGSI GetSalesForecast TELAH DIPERBARUI ---
func GetSalesForecast(c *gin.Context) {
	var requestBody struct {
		ProductName string `json:"product_name"`
		Periods     int    `json:"periods"`
		OutletID    uint   `json:"outlet_id"` // Ini akan 0 jika tidak dikirim
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request body tidak valid"})
		return
	}

	user, _ := c.Get("user")
	currentUser := user.(models.User) // User ini diambil dari DB (sesuai token)

	// Tentukan outlet_id yang digunakan
	var outletID uint
	if currentUser.Role == "owner" {
		// --- 🔹 PERBAIKAN VALIDASI 🔹 ---
		// 'requestBody.OutletID' dikirim dari frontend
		if requestBody.OutletID != 0 {
			outletID = requestBody.OutletID
		} else {
			// JIKA owner, TAPI 'outlet_id' adalah 0 (lupa pilih)
			// BLOKIR PERMINTAANNYA!
			c.JSON(http.StatusBadRequest, gin.H{"error": "Owner harus memilih outlet terlebih dahulu"})
			return
		}
		// --- 🔹 AKHIR PERBAIKAN 🔹 ---
	} else {
		// Untuk peran selain owner (Branch Manager, dll)
		// Kita pakai OutletID dari token/database mereka
		if currentUser.OutletID != nil {
			outletID = *currentUser.OutletID
		} else {
			// Jika admin (yang OutletID-nya juga null) mencoba, blokir juga
			c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya owner yang bisa memilih outlet, peran Anda tidak memiliki outlet."})
			return
		}
	}

	// Kirim ke service ML
	mlRequestBody := map[string]interface{}{
		"product_name": requestBody.ProductName,
		"outlet_id":    outletID, // 'outletID' sekarang PASTI valid
		"periods":      requestBody.Periods,
	}

	jsonData, _ := json.Marshal(mlRequestBody)

	// mlServiceURL := "http://127.0.0.1:5000/predict"
	mlServiceURL := "https://ceola-conjoined-undiaphanously.ngrok-free.dev/predict"
	resp, err := http.Post(mlServiceURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Gagal menghubungi layanan prediksi",
			"details": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	var predictionResponse interface{}
	if err := json.NewDecoder(resp.Body).Decode(&predictionResponse); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca respons dari layanan prediksi"})
		return
	}

	c.JSON(resp.StatusCode, predictionResponse)
}

// --- FUNGSI GetBasketAnalysis TELAH DIPERBARUI ---
func GetBasketAnalysis(c *gin.Context) {
	user, _ := c.Get("user")
	currentUser := user.(models.User)

	var outletID uint

	if currentUser.Role == "owner" {
		// Jika owner, WAJIB menyertakan ?outlet_id=... di URL
		outletIDStr := c.Query("outlet_id")
		if outletIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Owner harus memilih outlet (?outlet_id=...)"})
			return
		}
		parsedID, err := strconv.ParseUint(outletIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format outlet_id tidak valid"})
			return
		}
		outletID = uint(parsedID)
	} else {
		// Untuk peran lain, pakai OutletID mereka
		if currentUser.OutletID == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Outlet pengguna tidak ditemukan atau belum diatur",
			})
			return
		}
		outletID = *currentUser.OutletID // dereference pointer
	}

	results := make([]BasketAnalysisResult, 0)

	rawQuery := `
        SELECT
            m1.name AS product_1,
            m2.name AS product_2,
            COUNT(*) AS frequency
        FROM
            transaction_details AS td1
        INNER JOIN
            transaction_details AS td2 
            ON td1.transaction_id = td2.transaction_id 
            AND td1.menu_id < td2.menu_id
        INNER JOIN
            menus AS m1 ON td1.menu_id = m1.id
        INNER JOIN
            menus AS m2 ON td2.menu_id = m2.id
        INNER JOIN
            transactions AS t ON td1.transaction_id = t.id
        WHERE
            t.outlet_id = ? 
        GROUP BY
            product_1, product_2
        ORDER BY
            frequency DESC
        LIMIT 10;
    `

	rows, err := config.DB.Raw(rawQuery, outletID).Rows() // Gunakan outletID dinamis
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Gagal menjalankan query analisis",
			"details": err.Error(),
		})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var res BasketAnalysisResult
		if err := rows.Scan(&res.Product1, &res.Product2, &res.Frequency); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Gagal memindai hasil analisis",
				"details": err.Error(),
			})
			return
		}
		results = append(results, res)
	}

	c.JSON(http.StatusOK, results)
}

// --- FUNGSI GetBusyHours TELAH DIPERBARUI ---
func GetBusyHours(c *gin.Context) {
	user, _ := c.Get("user")
	currentUser := user.(models.User)

	var outletID uint

	if currentUser.Role == "owner" {
		// Jika owner, WAJIB menyertakan ?outlet_id=... di URL
		outletIDStr := c.Query("outlet_id")
		if outletIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Owner harus memilih outlet (?outlet_id=...)"})
			return
		}
		parsedID, err := strconv.ParseUint(outletIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format outlet_id tidak valid"})
			return
		}
		outletID = uint(parsedID)
	} else {
		// Untuk peran lain, pakai OutletID mereka
		if currentUser.OutletID == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Outlet pengguna tidak ditemukan atau belum diatur",
			})
			return
		}
		outletID = *currentUser.OutletID // dereference pointer
	}

	var results []BusyHourResult

	err := config.DB.Table("transactions").
		Select("DAYOFWEEK(created_at) as day_of_week, HOUR(created_at) as hour, COUNT(*) as count").
		Where("outlet_id = ?", outletID). // Gunakan outletID dinamis
		Group("day_of_week, hour").
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal melakukan analisis waktu sibuk", "details": err.Error()})
		return
	}

	if results == nil {
		results = make([]BusyHourResult, 0)
	}

	c.JSON(http.StatusOK, results)
}

// --- FUNGSI GetCustomerSegmentation TELAH DIPERBARUI ---
func GetCustomerSegmentation(c *gin.Context) {
	user, _ := c.Get("user")
	currentUser := user.(models.User)

	var outletID uint

	if currentUser.Role == "owner" {
		// Jika owner, WAJIB menyertakan ?outlet_id=... di URL
		outletIDStr := c.Query("outlet_id")
		if outletIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Owner harus memilih outlet (?outlet_id=...)"})
			return
		}
		parsedID, err := strconv.ParseUint(outletIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format outlet_id tidak valid"})
			return
		}
		outletID = uint(parsedID)
	} else {
		// Untuk peran lain, pakai OutletID mereka
		if currentUser.OutletID == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Outlet pengguna tidak ditemukan atau belum diatur",
			})
			return
		}
		outletID = *currentUser.OutletID // dereference pointer
	}

	var rfmData []CustomerRFM

	err := config.DB.Table("customers").
		Select(`
			customers.id as customer_id,
			customers.name,
			customers.phone_number,
			MAX(transactions.created_at) as last_purchase,
			COUNT(transactions.id) as frequency,
			SUM(transactions.total_amount) as monetary
		`).
		Joins("JOIN transactions ON transactions.customer_id = customers.id").
		Where("customers.outlet_id = ?", outletID). // Gunakan outletID dinamis
		Group("customers.id").
		Scan(&rfmData).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data RFM", "details": err.Error()})
		return
	}

	now := time.Now()
	for i := range rfmData {
		recencyDays := now.Sub(rfmData[i].LastPurchase).Hours() / 24

		if recencyDays <= 30 && rfmData[i].Frequency >= 5 && rfmData[i].Monetary > 500000 {
			rfmData[i].Segment = "Pelanggan Juara"
		} else if recencyDays <= 60 && rfmData[i].Frequency >= 3 {
			rfmData[i].Segment = "Pelanggan Setia"
		} else if recencyDays > 90 && rfmData[i].Frequency > 5 {
			rfmData[i].Segment = "Pelanggan Berisiko"
		} else if rfmData[i].Frequency == 1 {
			rfmData[i].Segment = "Pelanggan Baru"
		} else if recencyDays > 120 {
			rfmData[i].Segment = "Pelanggan Tidur"
		} else {
			rfmData[i].Segment = "Pelanggan Potensial"
		}
	}

	if rfmData == nil {
		rfmData = make([]CustomerRFM, 0)
	}

	c.JSON(http.StatusOK, rfmData)
}
