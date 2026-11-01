package main

import (
	"fmt"
	"inventory/internals/app"
	"inventory/internals/config"
	"inventory/internals/database"
	"inventory/internals/routes"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	start := time.Now()

	if err := config.InitConfig(); err != nil {
		log.Printf("%v", err)
	}

	if err := database.InitializeDatabase(); err != nil {
		log.Printf("db connection error : %v", err)
	}

	application, err := app.New()
	if err != nil {
		log.Fatalf("failed to initalize app : %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.HEAD("/", func(c *gin.Context) {
		fmt.Println("calling head ip : ", c.ClientIP(), c.RemoteIP())
		c.Status(200)
	})

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://gharmai-shikshya-tms.vercel.app", "https://gharmai-shikshya-tms-git-main-abhishekh-nepali.vercel.app", "https://gharmai-shikshya-li0cr1qzs-abhishekh-nepali.vercel.app"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "Cookie"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	routes.SetUpRoutes(router, application)
	log.Println("🌐 Starting HTTP server on :8080...")
	log.Println("📡 Server endpoints are now available")
	log.Println("App ready in", time.Since(start).Seconds(), "seconds.")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}

}
