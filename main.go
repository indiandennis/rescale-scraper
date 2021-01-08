package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	numQueryRoutines = 5 //this is to prevent overloading the website we're scraping, can increase for performance
)

type response struct {
	URL  string
	HTML string
}

func main() {
	//Validate starting URL
	if len(os.Args) != 2 {
		fmt.Println("Error: Run with one argument for the url to start crawling from.")
		os.Exit(1)
	}

	initialUrl, err := url.ParseRequestURI(os.Args[1])
	if err != nil {
		fmt.Println("Error: Invalid initial url, please format like https://google.com")
		os.Exit(1)
	}

	//setup channels to communicate between routines
	urlChannel := make(chan string, 65536) //this needs to be large to deal with exponential growth of link count (not ideal but works)
	responseChannel := make(chan response)

	urlChannel <- initialUrl.String()

	//start threads for parsing and querying
	for i := 0; i < numQueryRoutines; i++ {
		go fetchRoutine(urlChannel, responseChannel)
	}
	go parseRoutine(urlChannel, responseChannel)

	//automatically end after 30 seconds
	time.Sleep(30 * time.Second)
	fmt.Println("Time's up, ending execution.")
}

func fetchRoutine(urlChannel <-chan string, responseChannel chan<- response) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	for {
		URL := <-urlChannel
		resp, err := client.Get(URL)
		//check if response is a valid html document
		if err != nil ||
			resp.StatusCode != 200 ||
			!strings.HasPrefix(resp.Header.Get("content-type"), "text/html;") {
			//fmt.Println("Response failed")
			continue
		}

		htmlBytes, err := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			continue
		}

		responseChannel <- response{URL: URL, HTML: string(htmlBytes)}
	}
}

//takes responses, parses urls, logs them, and adds them to be crawled
func parseRoutine(urlChannel chan<- string, responseChannel <-chan response) {
	seenURLs := make(map[string]bool)
	for {
		response := <-responseChannel
		seenURLs[response.URL] = true

		document, err := goquery.NewDocumentFromReader(strings.NewReader(response.HTML))
		if err != nil {
			continue
		}

		//find links
		foundLinks := make(map[string]bool)
		document.Find("a").Each(func(i int, s *goquery.Selection) {
			link, exists := s.Attr("href")
			if exists {
				//only care about absolute urls
				URI, err := url.ParseRequestURI(link)
				if err == nil && URI.IsAbs() {
					foundLinks[URI.String()] = true
				}
			}
		})

		//print found links
		fmt.Println(response.URL)
		for link := range foundLinks {
			fmt.Println("\t" + link)
			if !seenURLs[link] {
				urlChannel <- link
				seenURLs[link] = true
			}
		}
	}
}
