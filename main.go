// websockets.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"server/ws"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
)

var hub *ws.Hub
var RedisClient *redis.Client

const (
	PlayerChannel = "Player-Channel"
)

func init() {
	password := os.Getenv("REDIS_PASSWORD")
	address := os.Getenv("REDIS_ADDRESS")
	RedisClient = redis.NewClient(&redis.Options{
		Addr:         address,
		MinIdleConns: 10,
		PoolSize:     100,
		IdleTimeout:  8 * time.Minute,
		ReadTimeout:  1 * time.Minute,
		WriteTimeout: 1 * time.Minute,
		PoolTimeout:  time.Duration(60) * time.Second,
		Password:     password, // no password set
		DB:           0,        // use default DB
	})
}

func main() {
	hub = ws.NewHub()
	go hub.Run()
	go ListenToPlayerChannelAndPublishToHub()

	router := gin.Default()
	router.Use(CORSMiddleware())

	router.LoadHTMLGlob("template/*.html")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "post.html", nil)
	})
	router.POST("/action", SendAction)
	router.GET("/ws/:id", wsServe)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("defaulting to port %s", port)
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
			return
		}
	}()

	fmt.Println("TCP Server is listening on port " + port)

	// Listen for the interrupt signal.
	<-ctx.Done()
	//<-quit

	// Restore default behavior on the interrupt signal and notify user of shutdown.
	stop()
	log.Println("Shutting down gracefully, press Ctrl+C again to force")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
}

func wsServe(c *gin.Context) {
	SessionId := c.Param("id")
	if SessionId == "" {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "id not found"})
		log.Println("id not found")
		return

	}
	ws.ServeWs(hub, SessionId, c.Writer, c.Request)
}
func SendAction(c *gin.Context) {
	var wsData ws.Data

	// Call BindJSON to bind the received JSON to
	// newAlbum.
	if err := c.BindJSON(&wsData); err != nil {
		log.Println("error :" + err.Error())
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error on json data not valid": err})
		return
	}
	if wsData.Action == "" {
		err := fmt.Errorf("action not found")
		log.Println("error :" + err.Error())
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err})
		return
	}
	if wsData.SessionId == "" {
		err := fmt.Errorf("session id not found")
		log.Println("error :" + err.Error())
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	switch wsData.Action {
	case "DropBall":
		wsData.Message = fmt.Sprintf("Action : %s ", wsData.Action)
		//ws.SendMessageToGame(hub, wsData)
		err := PublishMessageToPlayerChannel(wsData)
		if err != nil {
			log.Println("error :" + err.Error())
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
	case "ConFetti":
		wsData.Message = fmt.Sprintf("Action : %s ", wsData.Action)
		err := PublishMessageToPlayerChannel(wsData)
		if err != nil {
			log.Println("error :" + err.Error())
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
	case "PlaySound":
		if wsData.Sound == "" {
			err := fmt.Errorf("sound name not found")
			log.Println("error :" + err.Error())
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return

		}
		wsData.Message = fmt.Sprintf("Action : %s  , sound name : %s", wsData.Action, wsData.Sound)

		err := PublishMessageToPlayerChannel(wsData)
		if err != nil {
			log.Println("error :" + err.Error())
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
	case "Spawn":
		if wsData.SpawnName == "" {
			err := fmt.Errorf("spawn name not found")
			log.Println("error :" + err.Error())
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		if wsData.Quantity == 0 {
			err := fmt.Errorf("quantity not found or 0")
			log.Println("error :" + err.Error())
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		wsData.Message = fmt.Sprintf("Action : %s  , spawn name : %s, quantity : %s", wsData.Action, wsData.SpawnName, fmt.Sprint(wsData.Quantity))
		err := PublishMessageToPlayerChannel(wsData)
		if err != nil {
			log.Println("error :" + err.Error())
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
	case "RemoveActor":
		if wsData.SpawnName == "" {
			err := fmt.Errorf("spawn name not found")
			log.Println("error :" + err.Error())
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		wsData.Message = fmt.Sprintf("Action : %s  , actor name : %s", wsData.Action, wsData.SpawnName)
		err := PublishMessageToPlayerChannel(wsData)
		if err != nil {
			log.Println("error :" + err.Error())
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
	case "SetTimeOut":
		if wsData.Minutes == 0 {
			err := fmt.Errorf("minutes not found or 0")
			log.Println("error :" + err.Error())
			c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		wsData.Message = fmt.Sprintf("Action set game timeout : %s  , minutes : %s", wsData.Action, fmt.Sprint(wsData.Minutes))
		err := PublishMessageToPlayerChannel(wsData)
		if err != nil {
			log.Println("error :" + err.Error())
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

	default:
		err := fmt.Errorf("action not found")
		log.Println("error :" + err.Error())
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	// Add the new album to the slice.
	c.IndentedJSON(http.StatusOK, wsData)
}
func PublishMessageToPlayerChannel(data ws.Data) error {
	log.Println("error :" + data.Action)
	msg, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := RedisClient.Publish(PlayerChannel, msg).Err(); err != nil {
		return err
	}
	return nil
}

// ------------------------------------------------------------
func ListenToPlayerChannelAndPublishToHub() {
	subscriber := RedisClient.Subscribe(PlayerChannel)
	defer subscriber.Close()

	channel := subscriber.Channel()
	for msg := range channel {
		var data ws.Data
		if err := json.Unmarshal([]byte(msg.Payload), &data); err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println("Received message from " + msg.Channel + " channel.")
		fmt.Println(msg)
		ws.SendMessageToGame(hub, data)

	}
}
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}

}
