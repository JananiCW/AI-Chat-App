package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Message struct {
	Role    string `bson:"role" json:"role"`
	Content string `bson:"content" json:"content"`
}

type Chat struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Title     string             `bson:"title" json:"title"`
	Messages  []Message          `bson:"messages" json:"messages"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type SendMessageRequest struct {
	ChatID  string `json:"chatId"`
	Message string `json:"message"`
}

type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

type GeminiContent struct {
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

var chatsCollection *mongo.Collection

func main() {
	godotenv.Load()
	connectDB()

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	r.POST("/chats", createChatHandler)
	r.GET("/chats", getChatsHandler)
	r.GET("/chats/:id", getChatHandler)
	r.POST("/chats/:id/message", sendMessageHandler)
	r.DELETE("/chats/:id", deleteChatHandler)

	fmt.Println("🚀 Server running on http://localhost:3001")
	r.Run(":3001")
}

func connectDB() {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		panic(err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		panic("MongoDB not reachable!")
	}

	chatsCollection = client.Database("aichat").Collection("chats")
	fmt.Println("✅ Connected to MongoDB!")
}

func askGemini(messages []Message) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY missing")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", apiKey)

	var contents []GeminiContent
	for _, msg := range messages {
		role := "user"
		if msg.Role == "ai" {
			role = "model"
		}
		contents = append(contents, GeminiContent{
			Role:  role,
			Parts: []GeminiPart{{Text: msg.Content}},
		})
	}

	reqBody := GeminiRequest{Contents: contents}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	fmt.Println("📡 Gemini status:", resp.Status)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gemini error: %s", string(body))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", err
	}

	if len(geminiResp.Candidates) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

func createChatHandler(c *gin.Context) {
	chat := Chat{
		ID:        primitive.NewObjectID(),
		Title:     "New Chat",
		Messages:  []Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := chatsCollection.InsertOne(context.Background(), chat)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create chat"})
		return
	}

	c.JSON(http.StatusOK, chat)
}

func getChatsHandler(c *gin.Context) {
	opts := options.Find().SetSort(bson.M{"updatedAt": -1})
	cursor, err := chatsCollection.Find(context.Background(), bson.M{}, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch chats"})
		return
	}
	defer cursor.Close(context.Background())

	var chats []Chat
	cursor.All(context.Background(), &chats)
	c.JSON(http.StatusOK, chats)
}

func getChatHandler(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var chat Chat
	err = chatsCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&chat)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "chat not found"})
		return
	}

	c.JSON(http.StatusOK, chat)
}

func sendMessageHandler(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var chat Chat
	err = chatsCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&chat)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "chat not found"})
		return
	}

	userMessage := Message{Role: "user", Content: req.Message}
	chat.Messages = append(chat.Messages, userMessage)

	if len(chat.Messages) == 1 {
		title := req.Message
		if len(title) > 30 {
			title = title[:30] + "..."
		}
		chat.Title = title
	}

	aiText, err := askGemini(chat.Messages)
	if err != nil {
		fmt.Println("❌ Gemini error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	aiMessage := Message{Role: "ai", Content: aiText}
	chat.Messages = append(chat.Messages, aiMessage)
	chat.UpdatedAt = time.Now()

	_, err = chatsCollection.UpdateOne(
		context.Background(),
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"messages":  chat.Messages,
			"title":     chat.Title,
			"updatedAt": chat.UpdatedAt,
		}},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"userMessage": userMessage,
		"aiMessage":   aiMessage,
	})
}

func deleteChatHandler(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	chatsCollection.DeleteOne(context.Background(), bson.M{"_id": id})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}