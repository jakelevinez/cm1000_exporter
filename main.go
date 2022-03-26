package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"

	"github.com/PuerkitoBio/goquery"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	tokenURI  = "/GenieLogin.asp"
	loginURI  = "/goform/GenieLogin"
	scrapeURI = "/DocsisStatus.asp"
)

type Modem struct {
	Url    string
	User   string
	Pass   string
	login  string
	Client *http.Client
}

type webToken struct {
	Token string
}

type ScrapeData struct {
	Name string
}

func (modem *Modem) getToken() webToken {
	tokenURL := modem.Url + tokenURI
	client := modem.Client

	fmt.Printf("Get request on login url " + tokenURI + "\n")

	response, err := client.Get(tokenURL)

	if err != nil {
		log.Fatalln("Error fetching response. ", err)
	}

	fmt.Printf("Got response \n")

	defer response.Body.Close()

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatal("Error loading HTTP response body. ", err)
	}

	token, _ := document.Find("input[name='webToken']").Attr("value")

	webToken := webToken{
		Token: token,
	}

	return webToken
}

func (modem *Modem) loginFunc() {
	client := modem.Client

	fmt.Printf("getting token \n")

	webToken := modem.getToken()

	fmt.Printf("Got web token \n")

	loginURL := modem.Url + loginURI

	data := url.Values{
		"webToken":      {webToken.Token},
		"loginUsername": {modem.User},
		"loginPassword": {modem.Pass},
		"login":         {modem.login},
	}

	fmt.Printf("%+v\n", data)

	response, err := client.PostForm(loginURL, data)

	if err != nil {
		log.Fatalln(err)
	}

	defer response.Body.Close()

	_, err = ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalln(err)
	}
}

func (modem *Modem) getData() *goquery.Document {
	scrapeURL := modem.Url + scrapeURI
	client := modem.Client

	response, err := client.Get(scrapeURL)

	if err != nil {
		log.Fatalln("Error fetching response. ", err)
	}

	defer response.Body.Close()

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatal("Error loading HTTP response body. ", err)
	}

	return document

}

func main() {

	fmt.Printf("Initializing modem parameters \n")

	url, existsUrl := os.LookupEnv("MODEM_URL")
	user, existsUser := os.LookupEnv("MODEM_USER")
	pass, existsPass := os.LookupEnv("MODEM_PASS")
	port, existsPort := os.LookupEnv("EXPORT_PORT")

	if existsUrl {
		fmt.Printf("Found modem url from env var \n")
	} else {
		url = "http://192.168.100.1"
	}

	if existsUser {
		fmt.Printf("Found modem user from env var \n")
	} else {
		user = "admin"
	}

	if existsPass {
		fmt.Printf("Found modem pass from env var \n")
	} else {
		pass = "password"
	}

	if existsPort {
		fmt.Printf("Found modem port from env var \n")
	} else {
		port = "9527"
	}

	fmt.Printf("Initializing cookiejar \n")

	jar, _ := cookiejar.New(nil)

	fmt.Printf("Initialized cookiejar \n")

	currentModem := Modem{
		Url:    url,
		User:   user,
		Pass:   pass,
		login:  "1",
		Client: &http.Client{Jar: jar},
	}

	// scrape modem

	fmt.Printf("Logging in to modem \n")

	currentModem.loginFunc()

	fmt.Printf("Logged in to Modem \n")

	ScrapeData := currentModem.getData()

	fmt.Printf("Scraped data \n")

	downstreamTable := ScrapeData.Find("table[id='dsTable']")
	upstreamTable := ScrapeData.Find("table[id='usTable']")
	dsOFDMTable := ScrapeData.Find("table[id='d31dsTable']")
	usOFDMTable := ScrapeData.Find("table[id='d31usTable']")

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(port, nil))

}
