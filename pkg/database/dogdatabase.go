package database

import (
	"sort"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// DogService handles database actions to get dog pics
type DogService struct {
	dynamo *dynamodb.DynamoDB
	table  *string
}

type DogPic struct {
	Dog       *string
	Key       *string
	Timestamp int64
	URL       *string
}

func NewDogService(sess *session.Session, tablename string) *DogService {
	return &DogService{
		dynamo: dynamodb.New(sess),
		table:  aws.String(tablename),
	}
}

func (ds *DogService) GetAll() ([]DogPic, error) {
	scanOut, err := ds.dynamo.Scan(&dynamodb.ScanInput{
		TableName: ds.table,
	})
	if err != nil {
		return nil, err
	}
	var output []DogPic
	for _, r := range scanOut.Items {
		currentPic, err := newDogPic(r)
		if err != nil {
			return nil, err
		}
		output = append(output, currentPic)
	}
	sort.Slice(output, func(i, j int) bool {
		return (output[i].Timestamp < output[j].Timestamp)
	})
	output = reverseSlice(output)
	return output, nil
}

func newDogPic(i map[string]*dynamodb.AttributeValue) (DogPic, error) {
	timestamp, err := strconv.ParseInt(*i["timestamp"].N, 10, 64)
	if err != nil {
		return DogPic{}, err
	}
	pic := DogPic{
		Dog:       i["dog-name"].S,
		Key:       i["key"].S,
		Timestamp: timestamp,
		URL:       i["url"].S,
	}
	return pic, nil
}

func (ds *DogService) Add(dog DogPic) (*dynamodb.PutItemOutput, error) {
	res, err := ds.dynamo.PutItem(
		&dynamodb.PutItemInput{
			TableName: ds.table,
			Item: map[string]*dynamodb.AttributeValue{
				"dog-name": &dynamodb.AttributeValue{
					S: dog.Dog,
				},
				"timestamp": &dynamodb.AttributeValue{
					N: aws.String(strconv.FormatInt(dog.Timestamp, 10)),
				},
				"key": &dynamodb.AttributeValue{
					S: dog.Key,
				},
				"url": &dynamodb.AttributeValue{
					S: dog.URL,
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (ds *DogService) GetDog(dog string) ([]DogPic, error) {
	queryout, err := ds.dynamo.Query(&dynamodb.QueryInput{
		TableName:                ds.table,
		ExpressionAttributeNames: map[string]*string{"#pk": aws.String("dog-name")},
		KeyConditionExpression:   aws.String("#pk = :dogname"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":dogname": &dynamodb.AttributeValue{S: aws.String(dog)},
		},
	})
	if err != nil {
		return nil, err
	}
	var output []DogPic
	for _, i := range queryout.Items {
		currentPic, err := newDogPic(i)
		if err != nil {
			return nil, err
		}
		output = append(output, currentPic)
	}
	output = reverseSlice(output)
	return output, nil
}

func reverseSlice(s []DogPic) []DogPic {
	for i := len(s)/2 - 1; i >= 0; i-- {
		opp := len(s) - 1 - i
		s[i], s[opp] = s[opp], s[i]
	}
	return s
}
