package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/go-co-op/gocron"
	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Email struct {
		FromEmail string `yaml:"fromEmail", envconfig:"FROM_EMAIL"`
		ToEmail   string `yaml:"toEmail", envconfig:"TO_EMAIL"`
		EmailHost string `yaml:"emailHost", envconfig:"EMAIL_HOST"`
		EmailUser string `yaml:"emailUser", envconfig:"EMAIL_USER"`
		EmailPW   string `yaml:"emailPW", envconfig:"EMAIL_PW"`
	} `yaml:"email"`
}

type response struct {
	SearchResponseModel searchResponseModel `json:"searchResponseModel"`
}

type searchResponseModel struct {
	ResultList resultList `json:"resultlist.resultlist"`
}

type resultList struct {
	Paging            paging              `json:"paging"`
	ResultlistEntries []resultlistEntries `json:"resultlistEntries"`
}

type paging struct {
	PageNumber       int `json:"pageNumber"`
	PageSize         int `json:"pageSize"`
	NumberOfPages    int `json:"numberOfPages"`
	NumberOfHits     int `json:"numberOfHits"`
	NumberOfListings int `json:"numberOfListings"`
}

type resultlistEntries struct {
	ResultlistEntry []resultlistEntry `json:"resultlistEntry"`
}

type resultlistEntry struct {
	ID          string     `json:"@id"`
	PublishDate string     `json:"@publishDate"`
	RealEstate  realEstate `json:"resultlist.realEstate"`
}

type realEstate struct {
	ID            string   `json:"@id"`
	Title         string   `json:"title"`
	ColdRent      coldRent `json:"price"`
	WarmRent      warmRent `json:"calculatedTotalRent"`
	LivingSpace   float32  `json:"livingSpace"`
	NumberOfRooms float32  `json:"numberOfRooms"`
}

type coldRent struct {
	Value    float32 `json:"value"`
	Currency string  `json:"currency"`
}

type warmRent struct {
	Rent coldRent `json:"totalRent"`
}

type offer struct {
	ID   string
	Rent float32
	Size float32
	Room float32
	Link string
}

func main() {
	// var cfg Config
	// readFile(&cfg)
	// readEnv(&cfg)

	// mail := gomail.NewMessage()
	// mail.SetHeader("From", cfg.Email.FromEmail)
	// mail.SetHeader("To", cfg.Email.ToEmail)
	// mail.SetHeader("Subject", "New Flat Found!")
	// d := gomail.NewDialer(cfg.Email.EmailHost, 2525, cfg.Email.EmailUser, cfg.Email.EmailPW)

	m := make(map[string]offer)
	firstRun := true

	s := gocron.NewScheduler(time.UTC)
	s.Every(1).Minutes().Do(func() {
		var offers = getAllListings()
		for i := 0; i < len(offers); i++ {
			_, found := m[offers[i].ID]
			if found {
				fmt.Printf("Already exists: %s \n", offers[i].Link)
				continue
			}

			m[offers[i].ID] = offers[i]
			fmt.Printf("New listing found: %s \n", offers[i].Link)

			if !firstRun {
				// mail.SetBody("text/html", offers[i].Link)
				// if err := d.DialAndSend(mail); err != nil {
				// 	panic(err)
				// }

				err := beeep.Notify("ImmoTrakt", "New flat found", "assets/information.png")
				if err != nil {
					panic(err)
				}
			}
		}
		firstRun = false
	})
	s.StartBlocking()
}

func getAllListings() []offer {
	numberOfPages := 1
	offers := make([]offer, 0, 1000)
	for i := 1; i <= numberOfPages; i++ {
		var resultList resultList = requestPage(i)
		numberOfPages = resultList.Paging.NumberOfPages
		results := resultList.ResultlistEntries[0].ResultlistEntry
		for i := 0; i < len(results); i++ {
			entry := results[i]
			id := entry.ID

			rent := entry.RealEstate.WarmRent.Rent.Value
			size := entry.RealEstate.LivingSpace
			room := entry.RealEstate.NumberOfRooms
			title := entry.RealEstate.Title

			wbsOffer := strings.Contains(strings.ToLower(title), "wbs")
			tauschOffer := strings.Contains(strings.ToLower(title), "tausch")
			maxWarmRent := float32(1000)

			if !wbsOffer && !tauschOffer && rent < maxWarmRent {
				offers = append(offers, offer{ID: id, Rent: rent, Size: size, Room: room, Link: fmt.Sprintf("https://www.immobilienscout24.de/expose/%s", id)})
			}
		}
	}

	sort.Slice(offers, func(i, j int) bool {
		return offers[i].Rent < offers[j].Rent
	})

	return offers
}

func requestPage(pageNumber int) resultList {
	// Let's start with a base url
	baseUrl, err := url.Parse("https://www.immobilienscout24.de")
	if err != nil {
		fmt.Println("Malformed URL: ", err.Error())
		panic(err)
	}

	// Add a Path Segment (Path segment is automatically escaped)
	baseUrl.Path += "Suche/shape/wohnung-mieten"

	// Prepare Query Parameters
	params := url.Values{}
	params.Add("petsallowedtypes", "yes,negotiable")
	params.Add("numberofrooms", "1.5-")
	params.Add("price", "-1000.0")
	params.Add("pricetype", "calculatedtotalrent")
	params.Add("livingspace", "50.0-")
	params.Add("equipment", "builtinkitchen")
	params.Add("shape", "d2h2X0llcW5wQWpLb0V4SGlEeGZBfWRCYF1nfkB8T2F6QHBBZWlCZ0JzYEJ1akBvb0FlV29yQGVVX051TGRQc1BiUHNOakxfT2RIaU50WGleeGdAb2dAZF9Aa2lAbmNAe0RobUFxQnBzQHhGbGBBZnBAdmlCdFt_cUBsWGxc")
	params.Add("pagenumber", strconv.Itoa(pageNumber))

	// Add Query Parameters to the URL
	baseUrl.RawQuery = params.Encode() // Escape Query Parameters

	fmt.Println(baseUrl.String())
	resp, err := http.Post(baseUrl.String(), "application/json", nil)
	if err != nil {
		panic(err)
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)

	response := response{}
	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		panic(err)
	}
	return response.SearchResponseModel.ResultList
}

func readFile(cfg *Config) {
	f, err := os.Open("config.yml")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		panic(err)
	}
}

func readEnv(cfg *Config) {
	err := envconfig.Process("", cfg)
	if err != nil {
		panic(err)
	}
}
