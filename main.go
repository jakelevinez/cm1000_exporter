package main

import (
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
	loginURL = "/GenieLogin.asp"
)

type Modem struct {
	Url    string `default:"http://192.168.100.1"`
	User   string `default:"admin"`
	Pass   string `default:"password"`
	port   string `default:"9527"`
	Client *http.Client
}

type webToken struct {
	Token string
}

type Scrape struct {
	Name string
}

func (modem *Modem) getToken() webToken {
	loginURL := modem.Url + loginURL
	client := modem.Client

	response, err := client.Get(loginURL)

	if err != nil {
		log.Fatalln("Error fetching response. ", err)
	}

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

func (modem *Modem) login(class Modem) {
	client := modem.Client

	webToken := modem.getToken()

	loginURL := modem.Url + "/GenieLogin.asp"

	data := url.Values{
		"webToken":      {webToken.Token},
		"loginUsername": {modem.User},
		"loginPassword": {modem.Pass},
	}

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

func main() {

	//initialize the modem vars via env vars

	currentModem := Modem{}

	url, existsUrl := os.LookupEnv("MODEM_URL")
	user, existsUser := os.LookupEnv("MODEM_USER")
	pass, existsPass := os.LookupEnv("MODEM_PASS")
	port, existsPort := os.LookupEnv("EXPORT_PORT")
	if existsUrl {
		currentModem.Url = url
	}
	if existsUser {
		currentModem.User = user
	}
	if existsPass {
		currentModem.Pass = pass
	}
	if existsPort {
		currentModem.port = port
	}

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(currentModem.port, nil))

	// scrape modem

	jar, _ := cookiejar.New(nil)

	modem := Modem{
		Client: &http.Client{Jar: jar},
	}

	modem.login(currentModem)

}
