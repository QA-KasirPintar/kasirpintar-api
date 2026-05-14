package handler

import (
	"fmt"
	"net/http"
	"os"
	"sync"

	"kasirpintar-api/config"
	"kasirpintar-api/controllers"
	"kasirpintar-api/middlewares"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/midtrans/midtrans-go"
)

var (
	app  *gin.Engine
	once sync.Once
)

func setup() {
	// Load .env only for local development
	if os.Getenv("VERCEL") == "" {
		_ = godotenv.Load()
		fmt.Println("Loaded .env (local mode)")
	}

	// Database
	fmt.Println("Connecting to database...")
	config.ConnectDatabase()

	// Midtrans
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	if serverKey == "" {
		fmt.Println("WARNING: MIDTRANS_SERVER_KEY not set")
	} else {
		controllers.MidtransCoreAPI.New(serverKey, midtrans.Sandbox)
		fmt.Println("Midtrans client initialized (sandbox)")
	}

	// Gin Setup
	gin.SetMode(gin.ReleaseMode)
	app = gin.New()
	app.Use(gin.Recovery())
	app.Use(config.CORSMiddleware())

	// Health check
	app.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "API Running",
			"service": "KasirPintar API",
		})
	})

	api := app.Group("/api")
	{
		// Setup route (hanya bisa dipakai sekali - untuk buat admin pertama)
		api.POST("/setup", controllers.CreateFirstAdmin)

		// Public routes
		api.POST("/login", controllers.Login)
		api.POST("/forgot-password", controllers.ForgotPassword)
		api.POST("/reset-password", controllers.ResetPassword)

		// Webhook routes
		api.POST("/payment/notification", controllers.HandlePaymentNotification)
		api.POST("/payments/notification", controllers.HandlePaymentNotification)

		// Protected routes
		auth := api.Group("/")
		auth.Use(middlewares.RequireAuth)
		{
			auth.PATCH("/users/me/password", controllers.ChangePassword)

			cashierAndUp := middlewares.AuthorizeRole("admin", "owner", "branch_manager", "cashier")
			auth.GET("/management/menus", cashierAndUp, controllers.GetAllMenus)

			cashierRoutes := auth.Group("/cashier")
			cashierRoutes.Use(cashierAndUp)
			{
				cashierRoutes.POST("/transactions/preview", controllers.PreviewTransaction)
				cashierRoutes.POST("/transactions", controllers.CreateTransaction)
				cashierRoutes.GET("/transactions/status/*invoice", controllers.GetTransactionStatusByInvoice)
				cashierRoutes.GET("/customers", controllers.GetCustomers)
				cashierRoutes.POST("/customers", controllers.CreateCustomer)
			}

			auth.GET("/reports/transactions", cashierAndUp, controllers.GetTransactions)

			managerAndUp := middlewares.AuthorizeRole("admin", "owner", "branch_manager")

			managementRoutes := auth.Group("/management")
			managementRoutes.Use(managerAndUp)
			{
				managementRoutes.POST("/menus", controllers.CreateMenu)
				managementRoutes.PUT("/menus/:id", controllers.UpdateMenu)
				managementRoutes.DELETE("/menus/:id", controllers.DeleteMenu)
			}

			promoRoutes := auth.Group("/promotions")
			promoRoutes.Use(managerAndUp)
			{
				promoRoutes.POST("", controllers.CreatePromotion)
				promoRoutes.GET("", controllers.GetAllPromotions)
				promoRoutes.PATCH("/:id/status", controllers.UpdatePromotionStatus)
				promoRoutes.GET("/:id", controllers.GetPromotionByID)
			}

			reportRoutes := auth.Group("/reports")
			reportRoutes.Use(managerAndUp)
			{
				reportRoutes.GET("/dashboard", controllers.GetDashboardSummary)
				reportRoutes.POST("/forecast", controllers.GetSalesForecast)
				reportRoutes.GET("/basket-analysis", controllers.GetBasketAnalysis)
				reportRoutes.GET("/busy-hours", controllers.GetBusyHours)
				reportRoutes.GET("/customer-segmentation", controllers.GetCustomerSegmentation)
				reportRoutes.GET("/transactions/export", controllers.ExportTransactions)
			}

			userManagementRoutes := auth.Group("/users")
			userManagementRoutes.Use(managerAndUp)
			{
				userManagementRoutes.GET("", controllers.GetAllUsers)
				userManagementRoutes.POST("", controllers.RegisterUser)
				userManagementRoutes.PUT("/:id", controllers.UpdateUser)
				userManagementRoutes.DELETE("/:id", controllers.DeleteUser)
				userManagementRoutes.GET("/archived", controllers.GetArchivedUsers)
				userManagementRoutes.PATCH("/:id/restore", controllers.RestoreUser)
				userManagementRoutes.DELETE("/:id/permanent", controllers.PermanentDeleteUser)
				userManagementRoutes.GET("/managers", controllers.GetAvailableManagers)
			}

			ownerAndUp := middlewares.AuthorizeRole("admin", "owner")
			outletRoutes := auth.Group("/outlets")
			outletRoutes.Use(ownerAndUp)
			{
				outletRoutes.GET("", controllers.GetAllOutlets)
				outletRoutes.POST("", controllers.CreateOutlet)
				outletRoutes.PUT("/:id", controllers.UpdateOutlet)
				outletRoutes.DELETE("/:id", controllers.DeleteOutlet)
			}

			taxRoutes := auth.Group("/tax")
			taxRoutes.Use(ownerAndUp)
			{
				taxRoutes.GET("", controllers.GetAllTaxRates)
				taxRoutes.POST("", controllers.CreateTaxRate)
				taxRoutes.GET("/:id", controllers.GetTaxRateByID)
				taxRoutes.PUT("/:id", controllers.UpdateTaxRate)
				taxRoutes.DELETE("/:id", controllers.DeleteTaxRate)
			}
		}
	}
}

// Handler is the Vercel serverless function entry point
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(setup)
	app.ServeHTTP(w, r)
}
