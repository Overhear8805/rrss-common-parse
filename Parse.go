package parse

import (
	"crypto/sha1"
	"encoding/hex"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/mmcdole/gofeed"
)

type RrssFeed struct {
	Id        string
	FeedUrl   string
	FeedTitle string
	ItemTitle string
	ItemBody  string
	ItemUrl   string
	Created   time.Time
}

func Parse(url string) ([]RrssFeed, error) {
	// Verify input URL
	log.Println("Received feed url: ", url)

	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(url)

	if err != nil {
		return nil, err
	}

	// Build Feed objects
	feedItems := make([]RrssFeed, len(feed.Items))
	for _, item := range feed.Items {
		id, err := generateId(item)
		if err != nil {
			return nil, err
		}

		parsedFeed := RrssFeed{
			Id:        id,
			FeedUrl:   string(url),
			FeedTitle: string(feed.Title),
			ItemBody:  item.Description,
			ItemUrl:   item.Link,
			Created:   time.Now(),
		}
		feedItems = append(feedItems, parsedFeed)
	}
	return feedItems, nil
}

func hashContent(content string) string {
	hasher := sha1.New()
	hasher.Write([]byte(content))
	bytes := hasher.Sum(nil)
	return hex.EncodeToString(bytes[:])
}

func generateId(item *gofeed.Item) (string, error) {
	if len(item.GUID) > 0 {
		log.Println("Using provided GUID as id")
		return item.GUID, nil
	}

	id := hashContent(item.Description)
	if len(id) > 0 {
		log.Println("Using hashed content as id")
		return id, nil
	}

	log.Println("Falling back to generate UUID id")
	uuid, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return uuid.String(), nil
}
