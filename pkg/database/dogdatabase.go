package database

import (
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
	Tags      []*string
	URL       *string
}

func (d DogPic) GetDog() string {
	return *d.Dog
}
func (d DogPic) GetKey() string {
	return *d.Key
}
func (d DogPic) GetTimeStamp() int64 {
	return d.Timestamp
}
func (d DogPic) GetTags() []*string {
	return d.Tags
}
func (d DogPic) GetURL() string {
	return d.URL
}

func NewDogService(sess *session.Session, tablename string) *DogService {
	return &DogService{
		dynamo: dynamodb.New(sess),
		table:  aws.String(tablename),
	}
}

func (ds *DogService) GetAll() ([]DogPic, error) {
	items, err := ds.dynamo.Scan(&dynamodb.ScanInput{})
	if err != nil {
		return nil, err
	}
	var output []DogPic
	for _, r := range items.Items {
		timestamp, err := strconv.ParseInt(*r["timestamp"].N, 10, 64)
		if err != nil {
			return nil, err
		}
		currentPic := DogPic{
			Dog:       r["dog-name"].S,
			Key:       r["key"].S,
			Tags:      r["tags"].SS,
			Timestamp: timestamp,
			URL:       r["url"].S,
		}
		output = append(output, currentPic)
	}
	return output, nil
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
				"tags": &dynamodb.AttributeValue{
					SS: dog.Tags,
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
