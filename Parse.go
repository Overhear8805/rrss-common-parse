package parse

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"

	"github.com/mmcdole/gofeed"
)

var p = bluemonday.UGCPolicy()

var tr = &http.Transport{
	IdleConnTimeout: 5 * time.Second,
}

var client = &http.Client{
	Transport: tr,
}

type RrssFeed struct {
	Id        string
	FeedUrl   string
	FeedTitle string
	ItemTitle string
	ItemBody  string
	ItemUrl   string
	Published string
	Extended  string
	Created   time.Time
}

func Parse(url string) ([]RrssFeed, error) {
	// Verify input URL
	log.Println("Received feed url: ", url)

	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(url)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}
	log.Printf("Parsing %v", feed.Title)
	log.Printf("Found %v items in feed", len(feed.Items))

	// Build Feed objects
	feedItems := make([]RrssFeed, 0)
	sliceLength := len(feed.Items)
	var wg sync.WaitGroup
	wg.Add(sliceLength)
	for i := 0; i < sliceLength; i++ {
		go func(i int) {
			defer wg.Done()
			item := feed.Items[i]
			// Generate ID for the item
			id, err := generateId(item)
			if err != nil {
				log.Fatal("Failed to generate ID for item", err)
			}

			// Fetch full article
			var extended = ""
			itemUrl := item.Link
			if len(itemUrl) > 0 {
				log.Printf("Fetching extended article for '%s'", itemUrl)
				extended, err = GetExtendedArticle(itemUrl)
				if err != nil {
					extended = ""
					log.Println(err)
				}
			} else {
				log.Printf("Item has no link, skip fetching extended (id '%s', title '%s')", id, item.Title)
			}

			// Strip html from body and extended body
			item.Description = p.Sanitize(item.Description)
			extended = p.Sanitize(extended)

			// Put it in the array
			feedItems = append(feedItems, RrssFeed{
				Id:        id,
				FeedUrl:   string(url),
				FeedTitle: string(feed.Title),
				ItemBody:  item.Description,
				ItemUrl:   item.Link,
				Published: item.Published,
				Extended:  extended,
				Created:   time.Now(),
			})

			log.Printf("Id=%v : Url=%v : Title=%v Extended (char count)=%v Item no: %d/%d", id, string(url), string(feed.Title), len(extended), i, sliceLength)
		}(i)
	}

	wg.Wait()
	log.Printf("Parsed %v items in %s", len(feedItems), url)
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

func GetExtendedArticle(link string) (string, error) {
	response, err := http.Get(link)
	if err != nil {
		return "", err
	}

	if err != nil {
		log.Fatal(err)
	}

	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode <= 299 {
		doc, err := goquery.NewDocumentFromResponse(response)
		if err != nil {
			log.Printf("Failed to parse body as HTML")
			return "", err
		}

		article := ""
		doc.Find("article").Each(func(i int, s *goquery.Selection) {
			articleHtml, _ := s.Html() //underscore is an error
			sanitized := p.Sanitize(articleHtml)

			// Pick the largest article
			if len(sanitized) > len(article) {
				article = sanitized
			}
		})
		log.Printf("%s responded with status code %d. Body is %d chars long", link, response.StatusCode, len(article))
		return article, nil
	}
	return "", errors.New(fmt.Sprintf("Expected 2XX status code but received '%d'", response.StatusCode))
}
