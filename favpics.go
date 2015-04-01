package main

import (
	// "encoding/json"
	"fmt"
	"github.com/kurrik/oauth1a"
	"github.com/kurrik/twittergo"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	// "runtime"
	"strings"
	"sync"
	"time"
)

func downloader(url string, wg *sync.WaitGroup) {
	defer wg.Done()
	tmp := strings.Split(url, "/")
	fileName := tmp[len(tmp)-1]

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	ioutil.WriteFile(fileName, body, 0644)

}

func main() {
	var err error

	key := os.Getenv("TWITTER_KEY")
	secret := os.Getenv("TWITTER_SECRET")
	minwait := time.Duration(10) * time.Second

	config := &oauth1a.ClientConfig{
		ConsumerKey:    key,
		ConsumerSecret: secret,
	}
	client := twittergo.NewClient(config, nil)

	// url = "https://api.twitter.com/1.1/favorites/list.json"
	query := url.Values{}
	query.Set("screen_name", os.Getenv("TWITTER_SCREEN_NAME"))
	query.Set("count", "200")
	endpoint := fmt.Sprintf("https://api.twitter.com/1.1/favorites/list.json?%v", query.Encode())
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		fmt.Printf("Could not parse request: %v\n", err)
		os.Exit(1)
	}
	resp, err := client.SendRequest(req)
	if err != nil {
		fmt.Printf("Could not send request: %v\n", err)
		os.Exit(1)
	}
	results := &twittergo.Timeline{}
	if err = resp.Parse(results); err != nil {
		if rle, ok := err.(twittergo.RateLimitError); ok {
			dur := rle.Reset.Sub(time.Now()) + time.Second
			if dur < minwait {
				// Don't wait less than minwait.
				dur = minwait
			}
			msg := "Rate limited. Reset at %v. Waiting for %v\n"
			fmt.Printf(msg, rle.Reset, dur)
			time.Sleep(dur)
			// continue // Retry request.
		} else {
			fmt.Printf("Problem parsing response: %v\n", err)
		}
	}
	batch := len(*results)
	if batch == 0 {
		fmt.Printf("No more results, end of timeline.\n")
		os.Exit(1)
		// break
	}

	wg := new(sync.WaitGroup)

	for _, tweet := range *results {
		if tweet["entities"] == nil {
			continue
		}
		entities := tweet["entities"].(map[string]interface{})
		if entities["media"] == nil {
			continue
		}

		media := entities["media"].([]interface{})
		for i := 0; i < len(media); i++ {
			mediaN := media[i].(map[string]interface{})
			var media_url string
			if mediaN["media_url_https"] != nil {
				media_url = mediaN["media_url_https"].(string)
			} else if mediaN["media_url_http"] != nil {
				media_url = mediaN["media_url_http"].(string)
			} else if mediaN["media_url"] != nil {
				media_url = mediaN["media_url"].(string)
			} else {
				continue
			}

			wg.Add(1)
			go downloader(media_url, wg)
		}
		// max_id = tweet.Id() - 1
		// total += 1
	}
	fmt.Printf("Got %v Tweets", batch)
	if resp.HasRateLimit() {
		fmt.Printf(", %v calls available", resp.RateLimitRemaining())
	}
	fmt.Printf(".\n")

	wg.Wait()
}
