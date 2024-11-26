package main

import (
	//standard library imports

	"os"
	"time"

	// AWS SDK imports
	// Provides types for AWS Lambda events
	// AWS Lambda runtime support
	// AWS SDK configuration
	"github.com/aws/aws-sdk-go-v2/service/dynamodb" // DynamoDB client
)

// URLMapping represents the structure of our DynamoDB items
type URLMapping struct {
	ShortURL    string    `json:"short_url" dynamodbav:"short_url"`
	LongURL     string    `json:"long_url" dynamodbav:"long_url"`
	CreatedAt   time.Time `json:"created_at" dynamodbav:"created_at"`
	AccessCount int       `json:"access_count" dynamodbav:"access_count"`
}

// CreateURLRequest represents the expected JSON structure for POST requests
type CreateURLRequest struct {
	LongURL string `json:"long_url"`
}

// Global variables
var (
	tableName = os.Getenv("url-shortener") // DynamoDB table name from environment variable
	ddbClient *dynamodb.Client             //Dynamodb client instance

)
