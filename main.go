package main

import (
"log"
"os"

"github.com/AntipasBen23/fedey-backend/handlers"
"github.com/gin-contrib/cors"
"github.com/gin-gonic/gin"
"github.com/joho/godotenv"
)

func main() {
_ = godotenv.Load()

r := gin.Default()

r.Use(cors.New(cors.Config{
AllowOrigins:     []string{"*"},
AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
ExposeHeaders:    []string{"Content-Length"},
AllowCredentials: true,
}))

v1 := r.Group("/v1")
{
v1.POST("/analyze", handlers.AnalyzeHandler)
}

port := os.Getenv("PORT")
if port == "" {
port = "8080"
}

log.Printf("Server starting on port %s", port)
r.Run(":" + port)
}
