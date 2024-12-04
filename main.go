package main

import (
	// Standard library imports
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	// AWS SDK imports
	"github.com/aws/aws-lambda-go/events" // Provides types for AWS Lambda events
	"github.com/aws/aws-lambda-go/lambda" // AWS Lambda runtime support
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config" // AWS SDK configuration
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb" // DynamoDB client
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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
	tableName = os.Getenv("DYNAMODB_TABLE") // DynamoDB table name from environment variable
	ddbClient *dynamodb.Client              //Dynamodb client instance

)

//init is called automatically when lambda starts up
//Initializes dynamodb client

func init() {
	//Load AWS configuration from environment or credentials file
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal(err)
	}

	//create DynamoDB client
	ddbClient = dynamodb.NewFromConfig(cfg)
}

// handleRequest is the main Lambda handler function
// It routes requests based on HTTP method
func handleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	switch request.HTTPMethod {
	case "POST":
		return createShortURL(ctx, request) //Handle URL creation
	case "GET":
		return getOriginalURL(ctx, request) //Handle URL redirection
	default:
		return events.APIGatewayProxyResponse{
			StatusCode: 405,
			Body:       "Method not allowed",
		}, nil
	}
}

// createShortURL handles POST requests to create new short URLs
func createShortURL(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Parse the JSON request body
	var createReq CreateURLRequest
	err := json.Unmarshal([]byte(request.Body), &createReq)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       "Invalid request body",
		}, nil
	}

	// Generate a new short URL()
	shortURL := generateShortURL()
	// Create a new URLMapping object
	urlMapping := URLMapping{
		ShortURL:    shortURL,
		LongURL:     createReq.LongURL,
		CreatedAt:   time.Now(),
		AccessCount: 0,
	}

	// Convert the URLMapping to DynamoDB attribute values
	item, err := attributevalue.MarshalMap(urlMapping)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Error marshaling item",
		}, err
	}

	// Save item to DynamoDB
	_, err = ddbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &tableName,
		Item:      item,
	})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Error saving to DynamoDB",
		}, err
	}

	//Return the created URLMapping as JSON
	response, _ := json.Marshal(urlMapping)
	return events.APIGatewayProxyResponse{
		StatusCode: 201,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(response),
	}, nil
}

// getOriginalURL handles GET requests to redirect short URLs
func getOriginalURL(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get the short URL from the path parameters
	shortURL := request.PathParameters["shortURL"]

	//Create the dynamodb key for querying
	key, err := attributevalue.MarshalMap(map[string]string{
		"short_url": shortURL,
	})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Error creating key",
		}, err
	}

	//Get item from DynamoDB
	result, err := ddbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &tableName,
		Key:       key,
	})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Error querying DynamoDB",
		}, err
	}

	//Return 404 if URL not found
	if result.Item == nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       "URL not found",
		}, nil
	}

	//Convert DynamoDB item back to URLMapping struct
	var urlMapping URLMapping
	err = attributevalue.UnmarshalMap(result.Item, &urlMapping)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Error unmarshaling item",
		}, err
	}

	// Increment the access count asynchronously
	// Note: We don't wait for this to complete before redirecting
	_, err = ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        &tableName,
		Key:              key,
		UpdateExpression: aws.String("SET access_count + :inc"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inc": &types.AttributeValueMemberN{Value: "1"},
		},
	})

	if err != nil {
		log.Printf("Error updating access count :%v", err)
	}

	// Return a redirect response to the original URL
	return events.APIGatewayProxyResponse{
		StatusCode: 301, //HTTP 301 Moved Permanently
		Headers: map[string]string{
			"Location": urlMapping.LongURL, // This header causes the browser to redirect
		},
	}, nil

}

//generateShortURL creates a new short URL
// Uses a timestamp

func generateShortURL() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())[:8]
}

// main function starts the lambda
func main() {
	lambda.Start(handleRequest)
}
